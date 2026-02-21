// Package packages 提供内置套件管理功能
// 将内置 Go 模块映射为前端可识别的"套件"，提供套件列表、状态、前端资源服务等 API
package packages

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "packages"
	ModuleName    = "Package Manager"
	ModuleVersion = "1.0.0"
)

// Manifest 对应套件的 manifest.json 结构
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`

	Frontend *FrontendConfig `json:"frontend,omitempty"`
	Backend  *BackendConfig  `json:"backend,omitempty"`

	HasFrontend bool `json:"hasFrontend,omitempty"`
	HasBackend  bool `json:"hasBackend,omitempty"`
}

// FrontendConfig 前端配置
type FrontendConfig struct {
	Root      string `json:"root,omitempty"` // 前端文件根目录，默认 "frontend/dist"
	Entry     string `json:"entry,omitempty"`
	Title     string `json:"title,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	MinWidth  int    `json:"minWidth,omitempty"`
	MinHeight int    `json:"minHeight,omitempty"`
	Resizable *bool  `json:"resizable,omitempty"`
	Singleton bool   `json:"singleton,omitempty"`
}

// BackendConfig 后端配置
type BackendConfig struct {
	Binary      string            `json:"binary,omitempty"`
	Port        int               `json:"port,omitempty"`
	HealthCheck string            `json:"healthCheck,omitempty"`
	AutoStart   bool              `json:"autoStart,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// PackageInfo 套件信息（返回给前端）
type PackageInfo struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	Description    string          `json:"description"`
	Author         string          `json:"author"`
	Category       string          `json:"category"`
	Icon           string          `json:"icon"`
	State          string          `json:"state"` // running, stopped, error
	HasFrontend    bool            `json:"hasFrontend"`
	HasBackend     bool            `json:"hasBackend"`
	InstalledAt    string          `json:"installedAt"`
	AutoStart      bool            `json:"autoStart"`
	FrontendConfig *FrontendConfig `json:"frontendConfig,omitempty"`
	FrontendURL    string          `json:"frontendUrl,omitempty"`
}

// PackageDetail 套件详情
type PackageDetail struct {
	PackageInfo
	InstallPath string   `json:"installPath"`
	DataPath    string   `json:"dataPath"`
	BackendPort int      `json:"backendPort,omitempty"`
	Permissions []string `json:"permissions"`
	ServiceName string   `json:"serviceName,omitempty"`
}

// builtinMapping 内置模块映射：manifest ID → backend module ID
var builtinMapping = map[string]string{
	"ruizi-docker":      "docker",
	"ruizi-download":    "download",
	"ruizi-backup":      "backup",
	"ruizi-samba":       "samba",
	"rde-pkg-sync":      "sync",
	"ruizi-windows":     "windows",
}

// Module 套件管理模块
type Module struct {
	ctx       *module.Context
	registry  *module.Registry
	manifests map[string]*Manifest // manifest ID → manifest
	appDirs   map[string]string    // manifest ID → app directory path
	processes map[string]*exec.Cmd // manifest ID → running backend process
	mu        sync.RWMutex
}

// New 创建套件管理模块
func New(registry *module.Registry) *Module {
	return &Module{
		registry:  registry,
		manifests: make(map[string]*Manifest),
		appDirs:   make(map[string]string),
		processes: make(map[string]*exec.Cmd),
	}
}

func (m *Module) ID() string             { return ModuleID }
func (m *Module) Name() string           { return ModuleName }
func (m *Module) Version() string        { return ModuleVersion }
func (m *Module) Dependencies() []string { return nil }

// Init 初始化模块，扫描并加载 manifest
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	// 扫描套件目录加载 manifest
	m.loadManifests()

	// 如果没有从文件系统找到任何 manifest，使用内置模块的硬编码信息
	if len(m.manifests) == 0 {
		ctx.Logger.Info("no manifests found on disk, using builtin module definitions")
		m.loadBuiltinDefaults()
	}

	ctx.Logger.Info("packages module initialized",
		zap.Int("manifests_loaded", len(m.manifests)))
	return nil
}

// loadManifests 扫描多个可能的套件目录
func (m *Module) loadManifests() {
	// 可能的套件安装路径
	searchDirs := []string{
		"/usr/share/rde/apps",   // deb 安装的内置套件
		"/var/lib/rde/packages", // 用户安装的套件
		"/opt/rde/packages",     // 可选安装路径
	}

	// 也尝试从 data_dir 下查找
	if m.ctx != nil && m.ctx.Config != nil {
		dataDir := m.ctx.Config.GetString("data_dir")
		if dataDir != "" {
			searchDirs = append(searchDirs, filepath.Join(dataDir, "packages"))
		}
	}

	for _, baseDir := range searchDirs {
		m.scanDir(baseDir)
	}
}

// scanDir 扫描指定目录下的套件
func (m *Module) scanDir(baseDir string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		// 目录不存在不是错误
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(baseDir, entry.Name(), "manifest.json")
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			m.ctx.Logger.Debug("skip directory without valid manifest",
				zap.String("dir", entry.Name()),
				zap.Error(err))
			continue
		}

		m.mu.Lock()
		m.manifests[manifest.ID] = manifest
		m.appDirs[manifest.ID] = filepath.Join(baseDir, entry.Name())
		m.mu.Unlock()

		m.ctx.Logger.Info("loaded package manifest",
			zap.String("id", manifest.ID),
			zap.String("dir", filepath.Join(baseDir, entry.Name())))
	}
}

// loadBuiltinDefaults 加载内置模块的默认定义
// 当文件系统上找不到 manifest.json 时使用
func (m *Module) loadBuiltinDefaults() {
	builtins := []Manifest{
		{
			ID: "ruizi-docker", Name: "Docker 应用", Version: "1.0.0",
			Description: "Docker 容器管理", Author: "liduanjun <duanjunzi@gmail.com>",
			Category: "system", Icon: "icon.svg",
			Frontend:    &FrontendConfig{Entry: "index.html", Title: "Docker 应用", Width: 640, Height: 480, Singleton: true},
			Backend:     &BackendConfig{Port: 18083, HealthCheck: "/health", AutoStart: true},
			HasFrontend: true, HasBackend: true,
		},
		{
			ID: "ruizi-download", Name: "下载管理", Version: "1.0.0",
			Description: "下载管理工具", Author: "liduanjun <duanjunzi@gmail.com>",
			Category: "tools", Icon: "icon.svg",
			Frontend:    &FrontendConfig{Entry: "index.html", Title: "下载管理", Width: 900, Height: 600, Singleton: true},
			Backend:     &BackendConfig{Port: 18084, HealthCheck: "/health", AutoStart: true},
			HasFrontend: true, HasBackend: true,
		},
		{
			ID: "ruizi-backup", Name: "备份管理", Version: "1.0.0",
			Description: "数据备份管理", Author: "liduanjun <duanjunzi@gmail.com>",
			Category: "system", Icon: "icon.svg",
			Frontend:    &FrontendConfig{Entry: "index.html", Title: "备份管理", Width: 900, Height: 650, Singleton: true},
			Backend:     &BackendConfig{Port: 18085, HealthCheck: "/health", AutoStart: true},
			HasFrontend: true, HasBackend: true,
		},
		{
			ID: "ruizi-samba", Name: "Samba 共享管理", Version: "1.0.0",
			Description: "SMB/CIFS 文件共享服务管理", Author: "liduanjun <duanjunzi@gmail.com>",
			Category: "network", Icon: "icon.svg",
			Frontend:    &FrontendConfig{Root: "frontend/dist", Entry: "index.html", Title: "Samba 共享管理", Width: 900, Height: 650, Singleton: true},
			Backend:     &BackendConfig{Port: 18081, HealthCheck: "/health", AutoStart: true},
			HasFrontend: true, HasBackend: true,
		},
		{
			ID: "rde-pkg-sync", Name: "文件同步", Version: "1.0.0",
			Description: "TUS 文件同步服务", Author: "liduanjun <duanjunzi@gmail.com>",
			Category: "tools", Icon: "icon.svg",
			Frontend:    &FrontendConfig{Entry: "index.html", Title: "文件同步", Width: 900, Height: 650, Singleton: true},
			HasFrontend: true, HasBackend: false,
		},
	}

	for i := range builtins {
		b := &builtins[i]
		m.mu.Lock()
		if _, exists := m.manifests[b.ID]; !exists {
			m.manifests[b.ID] = b
			// 不设置 appDirs，前端资源将回退到主前端中的嵌入页面
		}
		m.mu.Unlock()
	}
}

// loadManifest 读取并解析 manifest.json
func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %w", err)
	}

	if manifest.ID == "" {
		return nil, fmt.Errorf("manifest missing id field")
	}

	// 根据 frontend/backend 配置推断 hasFrontend/hasBackend
	if manifest.Frontend != nil {
		manifest.HasFrontend = true
	}
	if manifest.Backend != nil {
		manifest.HasBackend = true
	}

	return &manifest, nil
}

func (m *Module) Start() error {
	m.ctx.Logger.Info("packages module started")
	m.startBackends()
	return nil
}

func (m *Module) Stop() error {
	m.ctx.Logger.Info("packages module stopping, shutting down backends...")
	m.stopBackends()
	m.ctx.Logger.Info("packages module stopped")
	return nil
}

// startBackends 启动所有配置了 autoStart 的套件后端进程
func (m *Module) startBackends() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, manifest := range m.manifests {
		appDir := m.appDirs[id]
		if appDir == "" || manifest.Backend == nil {
			continue
		}
		if !manifest.Backend.AutoStart {
			continue
		}
		if manifest.Backend.Binary == "" || manifest.Backend.Port == 0 {
			continue
		}

		binaryPath := filepath.Join(appDir, manifest.Backend.Binary)
		if _, err := os.Stat(binaryPath); err != nil {
			m.ctx.Logger.Warn("backend binary not found, skip",
				zap.String("id", id),
				zap.String("binary", binaryPath))
			continue
		}

		go m.startProcess(id, manifest, appDir, binaryPath)
	}
}

// startProcess 启动单个套件后端进程
func (m *Module) startProcess(id string, manifest *Manifest, appDir, binaryPath string) {
	cmd := exec.Command(binaryPath)
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 设置环境变量
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", manifest.Backend.Port),
		fmt.Sprintf("DATA_DIR=/var/lib/rde/apps/%s", filepath.Base(appDir)),
	)
	if manifest.Backend.Env != nil {
		for k, v := range manifest.Backend.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if err := cmd.Start(); err != nil {
		m.ctx.Logger.Error("failed to start backend process",
			zap.String("id", id),
			zap.String("binary", binaryPath),
			zap.Error(err))
		return
	}

	m.mu.Lock()
	m.processes[id] = cmd
	m.mu.Unlock()

	m.ctx.Logger.Info("backend process started",
		zap.String("id", id),
		zap.Int("pid", cmd.Process.Pid),
		zap.Int("port", manifest.Backend.Port))

	// 等待进程结束（后台监控）
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		delete(m.processes, id)
		m.mu.Unlock()
		if err != nil {
			m.ctx.Logger.Warn("backend process exited with error",
				zap.String("id", id),
				zap.Error(err))
		} else {
			m.ctx.Logger.Info("backend process exited",
				zap.String("id", id))
		}
	}()

	// 等待进程就绪（健康检查）
	if manifest.Backend.HealthCheck != "" {
		m.waitForHealthy(id, manifest.Backend.Port, manifest.Backend.HealthCheck)
	}
}

// waitForHealthy 等待后端进程健康检查通过
func (m *Module) waitForHealthy(id string, port int, path string) {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				m.ctx.Logger.Info("backend process healthy",
					zap.String("id", id),
					zap.Int("port", port))
				return
			}
		}
	}
	m.ctx.Logger.Warn("backend process health check timed out",
		zap.String("id", id),
		zap.Int("port", port))
}

// stopBackends 停止所有运行中的套件后端进程
func (m *Module) stopBackends() {
	m.mu.Lock()
	procs := make(map[string]*exec.Cmd)
	for k, v := range m.processes {
		procs[k] = v
	}
	m.mu.Unlock()

	for id, cmd := range procs {
		if cmd.Process != nil {
			m.ctx.Logger.Info("stopping backend process",
				zap.String("id", id),
				zap.Int("pid", cmd.Process.Pid))
			// 先发 SIGTERM 优雅停止
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			// 等待 5 秒后强制杀
			go func(c *exec.Cmd, name string) {
				time.Sleep(5 * time.Second)
				if c.ProcessState == nil || !c.ProcessState.Exited() {
					_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
					m.ctx.Logger.Warn("force killed backend process", zap.String("id", name))
				}
			}(cmd, id)
		}
	}
}

// RegisterRoutes 注册套件管理 API
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	pkg := group.Group("/packages")
	{
		pkg.GET("", m.handleList)
		pkg.GET("/:id", m.handleGet)
		pkg.POST("/:id/start", m.handleStart)
		pkg.POST("/:id/stop", m.handleStop)
		pkg.POST("/:id/restart", m.handleRestart)
		pkg.PUT("/:id/autostart", m.handleSetAutoStart)
	}

	// 套件前端资源服务
	group.GET("/pkg-assets/:id/*filepath", m.handleAssets)

	// 套件图标
	group.GET("/pkg-icon/:id", m.handleIcon)

	// 套件后端 API 代理：将请求转发到套件的独立后端进程
	group.Any("/pkg-api/:id/*path", m.handleProxyAPI)
}

// handleList 获取所有套件列表
func (m *Module) handleList(c *gin.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	packages := make([]PackageInfo, 0)

	for manifestID, manifest := range m.manifests {
		state := m.getPackageState(manifestID)
		pkg := PackageInfo{
			ID:          manifest.ID,
			Name:        manifest.Name,
			Version:     manifest.Version,
			Description: manifest.Description,
			Author:      manifest.Author,
			Category:    manifest.Category,
			Icon:        fmt.Sprintf("/api/v1/pkg-icon/%s", manifest.ID),
			State:       state,
			HasFrontend: manifest.HasFrontend,
			HasBackend:  manifest.HasBackend,
			InstalledAt: "2025-01-01T00:00:00Z", // 内置套件
			AutoStart:   true,
		}

		if manifest.Frontend != nil {
			pkg.FrontendConfig = manifest.Frontend
		}

		packages = append(packages, pkg)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    packages,
	})
}

// handleGet 获取套件详情
func (m *Module) handleGet(c *gin.Context) {
	id := c.Param("id")

	m.mu.RLock()
	manifest, ok := m.manifests[id]
	appDir := m.appDirs[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "套件不存在",
		})
		return
	}

	state := m.getPackageState(id)
	detail := PackageDetail{
		PackageInfo: PackageInfo{
			ID:          manifest.ID,
			Name:        manifest.Name,
			Version:     manifest.Version,
			Description: manifest.Description,
			Author:      manifest.Author,
			Category:    manifest.Category,
			Icon:        fmt.Sprintf("/api/v1/pkg-icon/%s", manifest.ID),
			State:       state,
			HasFrontend: manifest.HasFrontend,
			HasBackend:  manifest.HasBackend,
			InstalledAt: "2025-01-01T00:00:00Z",
			AutoStart:   true,
		},
		InstallPath: appDir,
		Permissions: []string{},
	}

	if manifest.Frontend != nil {
		detail.FrontendConfig = manifest.Frontend
	}
	if manifest.Backend != nil {
		detail.BackendPort = manifest.Backend.Port
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    detail,
	})
}

// handleStart 启动套件（内置模块已随主进程运行，返回成功即可）
func (m *Module) handleStart(c *gin.Context) {
	id := c.Param("id")

	m.mu.RLock()
	_, ok := m.manifests[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "套件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// handleStop 停止套件
func (m *Module) handleStop(c *gin.Context) {
	id := c.Param("id")

	m.mu.RLock()
	_, ok := m.manifests[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "套件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// handleRestart 重启套件
func (m *Module) handleRestart(c *gin.Context) {
	id := c.Param("id")

	m.mu.RLock()
	_, ok := m.manifests[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "套件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// handleSetAutoStart 设置自动启动
func (m *Module) handleSetAutoStart(c *gin.Context) {
	id := c.Param("id")

	m.mu.RLock()
	_, ok := m.manifests[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "套件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// handleAssets 服务套件前端资源文件
func (m *Module) handleAssets(c *gin.Context) {
	id := c.Param("id")
	filePath := c.Param("filepath")

	// 去掉前导 /
	filePath = strings.TrimPrefix(filePath, "/")
	if filePath == "" {
		filePath = "index.html"
	}

	m.mu.RLock()
	manifest, ok := m.manifests[id]
	appDir := m.appDirs[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "套件不存在"})
		return
	}

	// 如果没有 appDir（内置回退模式），返回一个最小化的占位页面
	if appDir == "" {
		m.serveBuiltinFallbackPage(c, manifest)
		return
	}

	// 确定前端文件根目录
	frontendRoot := "frontend/dist"
	if manifest.Frontend != nil && manifest.Frontend.Root != "" {
		frontendRoot = manifest.Frontend.Root
	}

	// 构建完整文件路径
	fullPath := filepath.Join(appDir, frontendRoot, filePath)

	// 安全检查：确保路径不逃出 appDir
	absPath, err := filepath.Abs(fullPath)
	if err != nil || !strings.HasPrefix(absPath, appDir) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid path"})
		return
	}

	// 检查文件是否存在
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		// SPA 回退：返回 index.html
		indexPath := filepath.Join(appDir, frontendRoot, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.File(indexPath)
			return
		}
		// 如果连 index.html 都没有，返回占位页面
		m.serveBuiltinFallbackPage(c, manifest)
		return
	}

	c.File(absPath)
}

// serveBuiltinFallbackPage 为尚未构建前端的内置模块返回一个占位页面
func (m *Module) serveBuiltinFallbackPage(c *gin.Context, manifest *Manifest) {
	title := manifest.Name
	if manifest.Frontend != nil && manifest.Frontend.Title != "" {
		title = manifest.Frontend.Title
	}

	// 找到对应后端模块ID，构建 API 基路径
	backendModuleID := ""
	if mid, ok := builtinMapping[manifest.ID]; ok {
		backendModuleID = mid
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #1a1a2e; color: #e0e0e0; display: flex; align-items: center; justify-content: center; min-height: 100vh; }
.container { text-align: center; padding: 40px; }
.icon { font-size: 64px; margin-bottom: 20px; }
h1 { font-size: 24px; margin-bottom: 12px; color: #fff; }
p { color: #888; font-size: 14px; margin-bottom: 8px; }
.api-hint { color: #666; font-size: 12px; margin-top: 20px; padding: 12px; background: #16213e; border-radius: 8px; }
code { color: #e2b714; }
</style>
</head>
<body>
<div class="container">
<div class="icon">📦</div>
<h1>%s</h1>
<p>%s</p>
<p style="color:#e2b714;">模块后端已运行，前端界面尚未部署</p>
<div class="api-hint">
<p>后端 API 路径: <code>/api/v1/%s/</code></p>
</div>
</div>
</body>
</html>`, title, title, manifest.Description, backendModuleID)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// handleIcon 服务套件图标
func (m *Module) handleIcon(c *gin.Context) {
	id := c.Param("id")

	m.mu.RLock()
	manifest, ok := m.manifests[id]
	appDir := m.appDirs[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "套件不存在"})
		return
	}

	// 如果有 appDir，尝试从中加载图标
	if appDir != "" {
		iconFile := manifest.Icon
		if iconFile == "" {
			iconFile = "icon.svg"
		}

		iconPath := filepath.Join(appDir, iconFile)
		if _, err := os.Stat(iconPath); err == nil {
			c.File(iconPath)
			return
		}
	}

	// 没有文件系统图标，返回一个简单的 SVG 占位图标
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
<rect width="64" height="64" rx="12" fill="#2d3748"/>
<text x="32" y="40" text-anchor="middle" font-size="28" fill="#e2b714">📦</text>
</svg>`
	c.Data(http.StatusOK, "image/svg+xml", []byte(svg))
}

// getPackageState 获取套件状态
func (m *Module) getPackageState(manifestID string) string {
	// 检查是否有外部后端进程在运行
	if _, running := m.processes[manifestID]; running {
		return "running"
	}

	// 检查内置模块映射
	backendModuleID, exists := builtinMapping[manifestID]
	if !exists {
		return "installed"
	}

	// 检查对应的后端模块是否已注册并运行
	if m.registry != nil {
		mod := m.registry.Get(backendModuleID)
		if mod != nil {
			return "running"
		}
	}

	return "stopped"
}

// handleProxyAPI 反向代理套件后端 API
// 将 /api/v1/pkg-api/:id/*path 转发到 http://127.0.0.1:{port}/*path
func (m *Module) handleProxyAPI(c *gin.Context) {
	id := c.Param("id")
	proxyPath := c.Param("path") // e.g. "/health", "/api/store/categories"

	m.mu.RLock()
	manifest, ok := m.manifests[id]
	m.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
		return
	}

	if manifest.Backend == nil || manifest.Backend.Port == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "package has no backend configured"})
		return
	}

	port := manifest.Backend.Port
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = proxyPath
			req.Host = target.Host
			// 保留原始 query string
			// req.URL.RawQuery 已经在原始请求中设置
		},
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
