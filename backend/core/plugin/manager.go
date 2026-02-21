package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// pluginInstance 插件运行时实例
type pluginInstance struct {
	manifest   Manifest
	dir        string // 插件目录
	socketPath string
	cmd        *exec.Cmd
	proxy      *httputil.ReverseProxy
	state      State
	errMsg     string
	startedAt  *time.Time
}

// Manager 插件管理器
// 负责发现、启动、停止插件，以及路由代理
type Manager struct {
	mu        sync.RWMutex
	pluginDir string // 插件安装目录，如 /var/lib/rde/plugins
	socketDir string // 插件 socket 目录，如 /var/run/rde/plugins
	dataDir   string // RDE 数据目录，如 /var/lib/rde
	baseDir   string // RDE 安装目录，如 /opt/rde
	debug     bool
	logger    *zap.Logger
	plugins   map[string]*pluginInstance
	stopCh    chan struct{}
}

// NewManager 创建插件管理器
func NewManager(pluginDir, socketDir, dataDir, baseDir string, debug bool, logger *zap.Logger) *Manager {
	return &Manager{
		pluginDir: pluginDir,
		socketDir: socketDir,
		dataDir:   dataDir,
		baseDir:   baseDir,
		debug:     debug,
		logger:    logger.Named("plugin"),
		plugins:   make(map[string]*pluginInstance),
		stopCh:    make(chan struct{}),
	}
}

// Discover 扫描插件目录，读取各插件的 manifest.json
func (m *Manager) Discover() error {
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Info("Plugin directory does not exist, no plugins to load",
				zap.String("dir", m.pluginDir))
			return nil
		}
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(m.pluginDir, entry.Name())
		if err := m.loadPlugin(pluginPath); err != nil {
			m.logger.Warn("Failed to load plugin",
				zap.String("path", pluginPath),
				zap.Error(err))
		}
	}

	m.logger.Info("Plugin discovery complete",
		zap.Int("count", len(m.plugins)))
	return nil
}

// loadPlugin 加载单个插件
func (m *Manager) loadPlugin(dir string) error {
	// 读取 manifest.json
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("invalid manifest JSON: %w", err)
	}

	if manifest.ID == "" {
		return fmt.Errorf("manifest missing required field 'id'")
	}
	if len(manifest.Routes) == 0 {
		return fmt.Errorf("manifest missing required field 'routes'")
	}

	// 检查二进制文件
	binary := manifest.Binary
	if binary == "" {
		binary = "plugin"
	}
	binaryPath := filepath.Join(dir, binary)
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("binary not found: %s", binaryPath)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary not executable: %s", binaryPath)
	}

	// 检查重复注册
	if _, exists := m.plugins[manifest.ID]; exists {
		return fmt.Errorf("plugin %s already loaded", manifest.ID)
	}

	socketPath := filepath.Join(m.socketDir, manifest.ID+".sock")

	// 为插件创建反向代理
	proxy := m.createProxy(manifest.ID, socketPath)

	m.plugins[manifest.ID] = &pluginInstance{
		manifest:   manifest,
		dir:        dir,
		socketPath: socketPath,
		proxy:      proxy,
		state:      StateStopped,
	}

	m.logger.Info("Plugin loaded",
		zap.String("id", manifest.ID),
		zap.String("name", manifest.Name),
		zap.String("version", manifest.Version),
		zap.Strings("routes", manifest.Routes))

	return nil
}

// createProxy 为插件创建 HTTP 反向代理
func (m *Manager) createProxy(id, socketPath string) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "plugin-" + id
		},
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", socketPath, 5*time.Second)
			},
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		FlushInterval: -1, // 立即刷新响应（支持 SSE）
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "plugin_unavailable",
				"message": fmt.Sprintf("Plugin '%s' is not responding", id),
			})
		},
	}
}

// StartAll 启动所有已发现的插件
func (m *Manager) StartAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, p := range m.plugins {
		if err := m.startPlugin(p); err != nil {
			m.logger.Error("Failed to start plugin",
				zap.String("id", id),
				zap.Error(err))
			p.state = StateError
			p.errMsg = err.Error()
		}
	}

	// 启动后台健康检查
	go m.healthCheckLoop()
}

// startPlugin 启动单个插件进程
func (m *Manager) startPlugin(p *pluginInstance) error {
	p.state = StateStarting

	// 确保 socket 目录存在
	if err := os.MkdirAll(m.socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// 清理旧 socket 文件
	os.Remove(p.socketPath)

	// 确定二进制路径
	binary := p.manifest.Binary
	if binary == "" {
		binary = "plugin"
	}
	binaryPath := filepath.Join(p.dir, binary)

	// 插件数据目录
	pluginDataDir := filepath.Join(m.dataDir, "plugin-data", p.manifest.ID)
	if err := os.MkdirAll(pluginDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin data directory: %w", err)
	}

	// 构建启动命令
	args := []string{
		"--socket", p.socketPath,
		"--data-dir", pluginDataDir,
		"--base-dir", m.baseDir,
	}
	if m.debug {
		args = append(args, "--debug")
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		p.state = StateError
		p.errMsg = err.Error()
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.cmd = cmd
	now := time.Now()
	p.startedAt = &now

	m.logger.Info("Plugin process started",
		zap.String("id", p.manifest.ID),
		zap.Int("pid", cmd.Process.Pid))

	// 等待 socket 就绪
	if err := m.waitForSocket(p.socketPath, 10*time.Second); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		p.state = StateError
		p.errMsg = "socket not ready: " + err.Error()
		return fmt.Errorf("socket not ready: %w", err)
	}

	// 健康检查
	if err := m.doHealthCheck(p); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		p.state = StateError
		p.errMsg = "health check failed: " + err.Error()
		return fmt.Errorf("health check failed: %w", err)
	}

	p.state = StateRunning
	m.logger.Info("Plugin started successfully",
		zap.String("id", p.manifest.ID),
		zap.Int("pid", cmd.Process.Pid))

	// 监控进程退出
	go m.monitorProcess(p)

	return nil
}

// waitForSocket 等待 Unix socket 就绪
func (m *Manager) waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", path, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for socket %s", path)
}

// doHealthCheck 执行插件健康检查
func (m *Manager) doHealthCheck(p *pluginInstance) error {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", p.socketPath, 2*time.Second)
			},
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://plugin/health")
	if err != nil {
		return fmt.Errorf("health request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("health check returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// monitorProcess 监控插件进程退出并自动重启
func (m *Manager) monitorProcess(p *pluginInstance) {
	if p.cmd == nil {
		return
	}

	// 阻塞等待进程退出
	err := p.cmd.Wait()

	m.mu.Lock()
	// 检查是否是正常停止（管理器主动关停）
	if p.state != StateRunning {
		m.mu.Unlock()
		return
	}

	errMsg := "process exited unexpectedly"
	if err != nil {
		errMsg = fmt.Sprintf("process exited: %v", err)
	}
	p.state = StateError
	p.errMsg = errMsg
	pluginID := p.manifest.ID
	m.mu.Unlock()

	m.logger.Error("Plugin process exited unexpectedly",
		zap.String("id", pluginID),
		zap.Error(err))

	// 延迟重启
	m.scheduleRestart(p)
}

// scheduleRestart 延迟重启插件
func (m *Manager) scheduleRestart(p *pluginInstance) {
	select {
	case <-m.stopCh:
		return // 管理器正在关闭
	case <-time.After(5 * time.Second):
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 再次检查管理器是否已关闭
	select {
	case <-m.stopCh:
		return
	default:
	}

	m.logger.Info("Restarting plugin", zap.String("id", p.manifest.ID))
	if err := m.startPlugin(p); err != nil {
		m.logger.Error("Failed to restart plugin",
			zap.String("id", p.manifest.ID),
			zap.Error(err))
	}
}

// healthCheckLoop 后台定期健康检查
func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			for _, p := range m.plugins {
				if p.state == StateRunning {
					if err := m.doHealthCheck(p); err != nil {
						m.logger.Warn("Plugin health check failed",
							zap.String("id", p.manifest.ID),
							zap.Error(err))
					}
				}
			}
			m.mu.RUnlock()
		}
	}
}

// HandleAPIRequest 处理插件 API 请求
// 如果请求匹配到插件路由则代理并返回 true，否则返回 false
func (m *Manager) HandleAPIRequest(c *gin.Context) bool {
	path := c.Request.URL.Path

	p := m.matchPluginAPI(path)
	if p == nil {
		return false
	}

	m.handleProxy(c, p)
	return true
}

// APIHandler 返回插件 API 路由处理器
// 作为 catch-all 路由使用，处理所有未被模块路由匹配的 API 请求
func (m *Manager) APIHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 查找匹配的插件
		p := m.matchPluginAPI(path)
		if p == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// 匹配到插件，执行代理
		m.handleProxy(c, p)
	}
}

// Middleware 返回 Gin 中间件，动态匹配插件 API 路由并代理请求
// 使用中间件而非静态路由注册，支持热插拔场景下的动态路由
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求路径中提取 /api/v1 之后的部分
		path := c.Request.URL.Path

		// 查找匹配的插件
		p := m.matchPluginAPI(path)
		if p == nil {
			// 没有匹配的插件，继续后续 handler
			c.Next()
			return
		}

		// 匹配到插件，执行代理
		m.handleProxy(c, p)
		c.Abort() // 阻止后续 handler
	}
}

// FrontendMiddleware 返回插件前端路由中间件
// 处理 /app/* 路由，代理到对应插件的前端服务
func (m *Manager) FrontendMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 查找匹配的插件前端
		p := m.matchPluginFrontend(path)
		if p == nil {
			c.Next()
			return
		}

		m.handleProxy(c, p)
		c.Abort()
	}
}

// matchPluginFrontend 匹配 /app/* 前端路由
func (m *Manager) matchPluginFrontend(path string) *pluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	const appPrefix = "/app/"
	if !strings.HasPrefix(path, appPrefix) {
		return nil
	}

	for _, p := range m.plugins {
		for _, app := range p.manifest.Apps {
			frontendRoute := strings.TrimSuffix(app.FrontendRoute, "/")
			if frontendRoute == "" {
				continue
			}
			// 精确前缀匹配
			if strings.HasPrefix(path, frontendRoute) {
				rest := path[len(frontendRoute):]
				if rest == "" || rest[0] == '/' {
					return p
				}
			}
		}
	}
	return nil
}

// matchPluginAPI 匹配 /api/v1/* API 路由
func (m *Manager) matchPluginAPI(path string) *pluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	const prefix = "/api/v1/"
	if !strings.HasPrefix(path, prefix) {
		return nil
	}
	subPath := path[len(prefix)-1:] // 保留开头的 "/"

	for _, p := range m.plugins {
		for _, route := range p.manifest.Routes {
			// 路由匹配规则：
			// "/android/*" 匹配 "/android" 和 "/android/xxx"
			// "/android" 精确匹配
			routePrefix := strings.TrimSuffix(route, "/*")
			routePrefix = strings.TrimSuffix(routePrefix, "*")

			if strings.HasPrefix(subPath, routePrefix) {
				// 确保是前缀匹配而非部分匹配
				// "/android" 应匹配 "/android" 和 "/android/xxx"，但不匹配 "/androidx"
				rest := subPath[len(routePrefix):]
				if rest == "" || rest[0] == '/' {
					return p
				}
			}
		}
	}
	return nil
}

// handleProxy 将请求代理到插件
func (m *Manager) handleProxy(c *gin.Context, p *pluginInstance) {
	m.mu.RLock()
	state := p.state
	m.mu.RUnlock()

	if state != StateRunning {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "plugin_unavailable",
			"message": fmt.Sprintf("Plugin '%s' is not available (state: %s)", p.manifest.ID, state),
		})
		return
	}

	// WebSocket 请求使用专门的代理
	if isWebSocketUpgrade(c.Request) {
		m.proxyWebSocket(c, p)
		return
	}

	// 普通 HTTP 请求使用 ReverseProxy
	p.proxy.ServeHTTP(c.Writer, c.Request)
}

// isWebSocketUpgrade 检测是否为 WebSocket 升级请求
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// proxyWebSocket 代理 WebSocket 连接
// 通过 TCP 层面的双向桥接实现透明代理
func (m *Manager) proxyWebSocket(c *gin.Context, p *pluginInstance) {
	// 连接到插件后端
	backendConn, err := net.DialTimeout("unix", p.socketPath, 5*time.Second)
	if err != nil {
		m.logger.Error("WebSocket proxy: failed to connect to plugin",
			zap.String("id", p.manifest.ID),
			zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to plugin"})
		return
	}

	// 将原始 HTTP 请求转发给后端
	if err := c.Request.Write(backendConn); err != nil {
		backendConn.Close()
		m.logger.Error("WebSocket proxy: failed to write request",
			zap.String("id", p.manifest.ID),
			zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to forward request"})
		return
	}

	// 劫持客户端连接
	hijacker, ok := c.Writer.(http.Hijacker)
	if !ok {
		backendConn.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "websocket hijack not supported"})
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		backendConn.Close()
		m.logger.Error("WebSocket proxy: hijack failed",
			zap.String("id", p.manifest.ID),
			zap.Error(err))
		return
	}

	// 双向桥接
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(backendConn, clientConn)
		// 关闭写端，通知对端已结束
		if tc, ok := backendConn.(*net.UnixConn); ok {
			tc.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		io.Copy(clientConn, backendConn)
		// 关闭写端
		if tc, ok := clientConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	wg.Wait()
	clientConn.Close()
	backendConn.Close()
}

// GetPlugins 获取所有插件信息（用于 API 响应）
func (m *Manager) GetPlugins() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]Info, 0, len(m.plugins))
	for _, p := range m.plugins {
		pid := 0
		if p.cmd != nil && p.cmd.Process != nil {
			pid = p.cmd.Process.Pid
		}
		infos = append(infos, Info{
			Manifest:  p.manifest,
			State:     p.state,
			Socket:    p.socketPath,
			PID:       pid,
			Error:     p.errMsg,
			StartedAt: p.startedAt,
		})
	}
	return infos
}

// GetPluginApps 获取所有运行中插件提供的前端应用
// 返回的列表用于前端动态注册插件应用到桌面环境
func (m *Manager) GetPluginApps() []PluginAppInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var apps []PluginAppInfo
	for _, p := range m.plugins {
		if p.state != StateRunning || len(p.manifest.Apps) == 0 {
			continue
		}
		for _, app := range p.manifest.Apps {
			apps = append(apps, PluginAppInfo{
				PluginID: p.manifest.ID,
				App:      app,
			})
		}
	}
	return apps
}

// HasPlugins 检查是否有已加载的插件
func (m *Manager) HasPlugins() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins) > 0
}

// GetPublicRoutes 返回所有插件声明的不需要认证的路由
func (m *Manager) GetPublicRoutes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var routes []string
	for _, p := range m.plugins {
		routes = append(routes, p.manifest.PublicRoutes...)
	}
	return routes
}

// Stop 停止所有插件
func (m *Manager) Stop() {
	// 通知后台 goroutine（健康检查、watcher）退出
	close(m.stopCh)

	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.plugins {
		m.stopPluginLocked(id)
	}

	m.logger.Info("All plugins stopped")
}
