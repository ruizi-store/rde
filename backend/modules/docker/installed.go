// Package docker 已安装应用服务
// 管理 Docker 应用的安装、卸载、启停、日志等生命周期操作
package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// composeCommand 检测可用的 docker compose 命令
// 优先使用 docker compose (V2)，不可用时回退到 docker-compose (V1)
func composeCommand(args ...string) *exec.Cmd {
	// 优先尝试 docker compose (V2 plugin)
	if checkCmd := exec.Command("docker", "compose", "version"); checkCmd.Run() == nil {
		fullArgs := append([]string{"compose"}, args...)
		return exec.Command("docker", fullArgs...)
	}
	// 回退到 docker-compose (V1)
	if path, err := exec.LookPath("docker-compose"); err == nil {
		return exec.Command(path, args...)
	}
	// 默认使用 docker compose（让它自己报错）
	fullArgs := append([]string{"compose"}, args...)
	return exec.Command("docker", fullArgs...)
}

// externalNetworkRegex 匹配 compose 中声明的 external network name
var externalNetworkRegex = regexp.MustCompile(`(?m)external:\s*true\s*\n\s*name:\s*(\S+)`)

// ensureExternalNetworks 自动创建 compose 文件中声明的 external 网络
func ensureExternalNetworks(compose string, logger *zap.Logger) {
	matches := externalNetworkRegex.FindAllStringSubmatch(compose, -1)
	seen := make(map[string]bool)
	for _, m := range matches {
		netName := m[1]
		if seen[netName] {
			continue
		}
		seen[netName] = true

		// 检查网络是否已存在
		check := exec.Command("docker", "network", "inspect", netName)
		if check.Run() == nil {
			continue // 已存在
		}

		// 创建网络
		logger.Info("Auto-creating external docker network", zap.String("network", netName))
		create := exec.Command("docker", "network", "create", netName)
		if output, err := create.CombinedOutput(); err != nil {
			logger.Warn("Failed to create docker network",
				zap.String("network", netName),
				zap.String("output", string(output)),
				zap.Error(err),
			)
		}
	}
}

// InstalledApp 已安装应用
type InstalledApp struct {
	Name        string                 `json:"name"`
	AppID       string                 `json:"app_id"`
	Version     string                 `json:"version"`
	Icon        string                 `json:"icon"`
	Status      string                 `json:"status"`
	Config      map[string]interface{} `json:"config"`
	ComposePath string                 `json:"compose_path"`
	InstalledAt time.Time              `json:"installed_at"`
}

// InstallTask 安装任务（异步）
type InstallTask struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // "pending", "running", "success", "failed"
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	DoneAt    time.Time `json:"done_at,omitempty"`
}

// InstalledService 已安装应用服务
type InstalledService struct {
	dataFile string // installed.json 路径
	appsDir  string // 应用安装目录
	catalog  *CatalogService
	apps     map[string]*InstalledApp
	mu       sync.RWMutex

	// 异步安装任务
	tasks   map[string]*InstallTask
	tasksMu sync.RWMutex

	// 容器状态缓存
	statusCache     map[string]string // container_name -> status
	statusCacheTime time.Time
	statusCacheTTL  time.Duration

	logger *zap.Logger
}

// NewInstalledService 创建已安装应用服务
func NewInstalledService(dataDir string, catalog *CatalogService, logger *zap.Logger) (*InstalledService, error) {
	appsDir := filepath.Join(dataDir, "docker-apps")
	dataFile := filepath.Join(dataDir, "docker-installed.json")

	// 确保目录存在
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create apps dir: %w", err)
	}

	is := &InstalledService{
		dataFile:       dataFile,
		appsDir:        appsDir,
		catalog:        catalog,
		apps:           make(map[string]*InstalledApp),
		tasks:          make(map[string]*InstallTask),
		statusCache:    make(map[string]string),
		statusCacheTTL: 5 * time.Second,
		logger:         logger,
	}

	// 加载已安装应用列表
	if err := is.load(); err != nil && !os.IsNotExist(err) {
		logger.Warn("Failed to load installed apps", zap.Error(err))
	}

	logger.Info("Installed apps service initialized",
		zap.String("apps_dir", appsDir),
		zap.Int("count", len(is.apps)),
	)

	return is, nil
}

// ==================== 持久化 ====================

func (is *InstalledService) load() error {
	is.mu.Lock()
	defer is.mu.Unlock()

	data, err := os.ReadFile(is.dataFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &is.apps)
}

// saveLocked 在已持有写锁的情况下保存（必须在 mu.Lock() 内调用）
func (is *InstalledService) saveLocked() error {
	dir := filepath.Dir(is.dataFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(is.apps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(is.dataFile, data, 0o644)
}

// ==================== 容器状态批量查询 ====================

// refreshStatusCache 一次 docker ps 获取所有容器状态
func (is *InstalledService) refreshStatusCache() {
	if time.Since(is.statusCacheTime) < is.statusCacheTTL {
		return
	}

	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}\t{{.State}}")
	output, err := cmd.Output()
	if err != nil {
		is.logger.Debug("docker ps failed", zap.Error(err))
		return
	}

	cache := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			cache[parts[0]] = parts[1]
		}
	}

	is.statusCache = cache
	is.statusCacheTime = time.Now()
}

// resolveStatus 从缓存中查找应用的容器状态
func (is *InstalledService) resolveStatus(app *InstalledApp) string {
	if app.ComposePath == "" {
		return "unknown"
	}

	is.refreshStatusCache()

	// 检查项目内的容器（名称一般以项目名为前缀或直接用 container_name）
	// 简单策略：查找名称包含 app.Name 的容器
	for name, state := range is.statusCache {
		if strings.Contains(name, app.Name) || strings.HasPrefix(name, app.Name) {
			if state == "running" {
				return "running"
			}
		}
	}

	// 如果缓存中没找到匹配，回退到 docker compose ps（只用于这一个app）
	cmd := composeCommand("-f", app.ComposePath, "ps", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	out := string(output)
	if strings.Contains(out, `"running"`) || strings.Contains(out, `"Running"`) {
		return "running"
	}
	if out == "" || out == "\n" {
		return "stopped"
	}
	return "stopped"
}

// ==================== 查询 ====================

// GetAll 获取所有已安装应用（含实时状态）
func (is *InstalledService) GetAll() []*InstalledApp {
	is.mu.RLock()
	appsCopy := make([]*InstalledApp, 0, len(is.apps))
	for _, app := range is.apps {
		cp := *app
		appsCopy = append(appsCopy, &cp)
	}
	is.mu.RUnlock()

	// 在锁外查询状态
	for _, app := range appsCopy {
		app.Status = is.resolveStatus(app)
	}
	return appsCopy
}

// Get 获取单个已安装应用
func (is *InstalledService) Get(name string) *InstalledApp {
	is.mu.RLock()
	app := is.apps[name]
	if app == nil {
		is.mu.RUnlock()
		return nil
	}
	cp := *app
	is.mu.RUnlock()

	cp.Status = is.resolveStatus(&cp)
	return &cp
}

// ==================== Compose 变量校验 ====================

var composeVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// detectUnreplacedVars 检测 compose 中未被替换的 ${...} 变量
func detectUnreplacedVars(compose string) []string {
	matches := composeVarRegex.FindAllStringSubmatch(compose, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, m := range matches {
		varName := m[1]
		if !seen[varName] {
			seen[varName] = true
			vars = append(vars, varName)
		}
	}
	return vars
}

// imageRegex 匹配 compose 中的 image: 行
var imageRegex = regexp.MustCompile(`(?m)^\s*image:\s*(.+)$`)

// extractImages 从 compose 内容中提取所有镜像名称
func extractImages(compose string) []string {
	matches := imageRegex.FindAllStringSubmatch(compose, -1)
	seen := make(map[string]bool)
	var images []string
	for _, m := range matches {
		img := strings.TrimSpace(m[1])
		// 去掉可能的引号
		img = strings.Trim(img, `"'`)
		if img == "" || seen[img] {
			continue
		}
		seen[img] = true
		images = append(images, img)
	}
	return images
}

// portRegex 匹配 compose 中 ports 段的端口映射行，如 "- 8080:3000" 或 "- 8080:3000/udp"
var portRegex = regexp.MustCompile(`(?m)^\s*-\s*"?(\d+)\s*:\s*\d+(?:/(?:tcp|udp))?"?\s*$`)

// extractHostPorts 从 compose 文本中提取所有主机端口
func extractHostPorts(compose string) []string {
	matches := portRegex.FindAllStringSubmatch(compose, -1)
	seen := make(map[string]bool)
	var ports []string
	for _, m := range matches {
		p := strings.TrimSpace(m[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		ports = append(ports, p)
	}
	return ports
}

// ==================== 安装 ====================

// Install 安装应用（同步）
func (is *InstalledService) Install(appID string, config map[string]interface{}) (*InstalledApp, string, error) {
	if is.catalog == nil {
		return nil, "", fmt.Errorf("app catalog not loaded")
	}

	appDef := is.catalog.GetApp(appID)
	if appDef == nil {
		return nil, "", fmt.Errorf("app not found: %s", appID)
	}

	// 应用实例名称
	name, _ := config["app_name"].(string)
	if name == "" {
		name = appID
	}

	// 检查是否已安装
	is.mu.RLock()
	_, exists := is.apps[name]
	is.mu.RUnlock()
	if exists {
		return nil, "", fmt.Errorf("应用已安装: %s", name)
	}

	// 版本
	version, _ := config["version"].(string)
	if version == "" {
		version = appDef.Version
	}

	// 创建应用目录
	appDir := filepath.Join(is.appsDir, name)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("创建应用目录失败: %w", err)
	}
	// 创建 data 子目录
	os.MkdirAll(filepath.Join(appDir, "data"), 0o755)

	// 生成 docker-compose.yml（变量替换）
	compose := appDef.Compose
	compose = strings.ReplaceAll(compose, "${version}", version)
	compose = strings.ReplaceAll(compose, "${app_name}", name)

	// 填充缺失的表单默认值
	for _, field := range appDef.Form {
		if _, ok := config[field.Key]; !ok && field.Default != nil {
			config[field.Key] = field.Default
		}
	}

	// 替换所有配置变量
	for key, value := range config {
		placeholder := fmt.Sprintf("${%s}", key)
		strVal := fmt.Sprintf("%v", value)
		compose = strings.ReplaceAll(compose, placeholder, strVal)
	}

	// 检测未替换的变量
	unreplaced := detectUnreplacedVars(compose)
	var warnings string
	if len(unreplaced) > 0 {
		warnings = fmt.Sprintf("⚠ 以下变量未配置，将使用占位符: %s\n", strings.Join(unreplaced, ", "))
		is.logger.Warn("Unreplaced compose variables",
			zap.String("app_id", appID),
			zap.Strings("vars", unreplaced),
		)
	}

	composePath := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		os.RemoveAll(appDir)
		return nil, "", fmt.Errorf("写入 compose 文件失败: %w", err)
	}

	// 检查 Docker
	if _, err := exec.LookPath("docker"); err != nil {
		os.RemoveAll(appDir)
		return nil, "", fmt.Errorf("Docker 未安装或不在 PATH 中")
	}

	// 自动创建 compose 中声明的 external 网络
	ensureExternalNetworks(compose, is.logger)

	// 启动容器
	is.logger.Info("Installing docker app",
		zap.String("app_id", appID),
		zap.String("name", name),
		zap.String("compose", composePath),
	)

	cmd := composeCommand("-f", composePath, "up", "-d")
	cmd.Dir = appDir
	output, err := cmd.CombinedOutput()
	outputStr := warnings + string(output)
	if err != nil {
		is.logger.Error("Docker compose up failed",
			zap.Error(err),
			zap.String("output", outputStr),
		)
		// 不删除目录，方便调试
		return nil, outputStr, fmt.Errorf("启动容器失败: %s", outputStr)
	}

	// 保存安装信息（在锁内保存）
	app := &InstalledApp{
		Name:        name,
		AppID:       appID,
		Version:     version,
		Icon:        appDef.Icon,
		Status:      "running",
		Config:      config,
		ComposePath: composePath,
		InstalledAt: time.Now(),
	}

	is.mu.Lock()
	is.apps[name] = app
	if err := is.saveLocked(); err != nil {
		is.logger.Error("Failed to save installed apps", zap.Error(err))
	}
	is.mu.Unlock()

	return app, outputStr, nil
}

// InstallStreaming 安装应用（流式输出版本）
// onOutput 回调在每读到新行时被调用，用于实时更新进度
func (is *InstalledService) InstallStreaming(appID string, config map[string]interface{}, onOutput func(string)) (*InstalledApp, string, error) {
	if is.catalog == nil {
		return nil, "", fmt.Errorf("app catalog not loaded")
	}

	appDef := is.catalog.GetApp(appID)
	if appDef == nil {
		return nil, "", fmt.Errorf("app not found: %s", appID)
	}

	name, _ := config["app_name"].(string)
	if name == "" {
		name = appID
	}

	is.mu.RLock()
	_, exists := is.apps[name]
	is.mu.RUnlock()
	if exists {
		return nil, "", fmt.Errorf("应用已安装: %s", name)
	}

	version, _ := config["version"].(string)
	if version == "" {
		version = appDef.Version
	}

	appDir := filepath.Join(is.appsDir, name)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("创建应用目录失败: %w", err)
	}
	os.MkdirAll(filepath.Join(appDir, "data"), 0o755)

	compose := appDef.Compose
	compose = strings.ReplaceAll(compose, "${version}", version)
	compose = strings.ReplaceAll(compose, "${app_name}", name)

	for _, field := range appDef.Form {
		if _, ok := config[field.Key]; !ok && field.Default != nil {
			config[field.Key] = field.Default
		}
	}
	for key, value := range config {
		placeholder := fmt.Sprintf("${%s}", key)
		strVal := fmt.Sprintf("%v", value)
		compose = strings.ReplaceAll(compose, placeholder, strVal)
	}

	unreplaced := detectUnreplacedVars(compose)
	var warnings string
	if len(unreplaced) > 0 {
		warnings = fmt.Sprintf("⚠ 以下变量未配置: %s\n", strings.Join(unreplaced, ", "))
	}

	composePath := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		os.RemoveAll(appDir)
		return nil, "", fmt.Errorf("写入 compose 文件失败: %w", err)
	}

	if _, err := exec.LookPath("docker"); err != nil {
		os.RemoveAll(appDir)
		return nil, "", fmt.Errorf("Docker 未安装或不在 PATH 中")
	}

	ensureExternalNetworks(compose, is.logger)

	// ---- 端口冲突预检 ----
	hostPorts := extractHostPorts(compose)
	var conflictPorts []string
	for _, p := range hostPorts {
		if !isPortAvailable(p) {
			conflictPorts = append(conflictPorts, p)
		}
	}
	if len(conflictPorts) > 0 {
		os.RemoveAll(appDir)
		return nil, "", fmt.Errorf("以下端口已被占用: %s，请更换端口后重试", strings.Join(conflictPorts, ", "))
	}

	is.logger.Info("Installing docker app (streaming)",
		zap.String("app_id", appID),
		zap.String("name", name),
	)

	var outputBuf strings.Builder
	if warnings != "" {
		outputBuf.WriteString(warnings)
	}

	// ---- 第一阶段：逐个拉取镜像（带进度百分比） ----
	images := extractImages(compose)
	if len(images) > 0 {
		for i, img := range images {
			if onOutput != nil {
				onOutput(fmt.Sprintf("正在拉取镜像 (%d/%d): %s", i+1, len(images), img))
			}
			outputBuf.WriteString(fmt.Sprintf("Pulling %s ...\n", img))

			pullCmd := exec.Command("docker", "pull", img)
			pullPr, pullPw := io.Pipe()
			pullCmd.Stdout = pullPw
			pullCmd.Stderr = pullPw

			if err := pullCmd.Start(); err != nil {
				pullPw.Close()
				pullPr.Close()
				is.logger.Warn("docker pull start failed", zap.String("image", img), zap.Error(err))
				continue
			}

			// 流式读取 pull 输出
			pullDone := make(chan struct{})
			go func() {
				defer close(pullDone)
				scanner := bufio.NewScanner(pullPr)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line == "" {
						continue
					}
					outputBuf.WriteString(line + "\n")
					if onOutput != nil {
						onOutput(fmt.Sprintf("[%d/%d] %s: %s", i+1, len(images), img, line))
					}
				}
			}()

			pullErr := pullCmd.Wait()
			pullPw.Close()
			<-pullDone

			if pullErr != nil {
				errMsg := fmt.Sprintf("拉取镜像失败: %s - %v", img, pullErr)
				outputBuf.WriteString(errMsg + "\n")
				return nil, outputBuf.String(), fmt.Errorf(errMsg)
			}

			if onOutput != nil {
				onOutput(fmt.Sprintf("镜像拉取完成 (%d/%d): %s ✓", i+1, len(images), img))
			}
		}
	}

	// ---- 第二阶段：启动容器 ----
	if onOutput != nil {
		onOutput("正在启动容器...")
	}

	cmd := composeCommand("-f", composePath, "up", "-d")
	cmd.Dir = appDir
	upOutput, err := cmd.CombinedOutput()
	outputBuf.WriteString(string(upOutput))

	if err != nil {
		outputStr := outputBuf.String()
		is.logger.Error("Docker compose up failed",
			zap.Error(err),
			zap.String("output", outputStr),
		)
		return nil, outputStr, fmt.Errorf("启动容器失败: %s", string(upOutput))
	}

	if onOutput != nil {
		onOutput("容器启动成功 ✓")
	}

	app := &InstalledApp{
		Name:        name,
		AppID:       appID,
		Version:     version,
		Icon:        appDef.Icon,
		Status:      "running",
		Config:      config,
		ComposePath: composePath,
		InstalledAt: time.Now(),
	}

	is.mu.Lock()
	is.apps[name] = app
	if err := is.saveLocked(); err != nil {
		is.logger.Error("Failed to save installed apps", zap.Error(err))
	}
	is.mu.Unlock()

	return app, outputBuf.String(), nil
}

// InstallAsync 异步安装应用，返回任务 ID
func (is *InstalledService) InstallAsync(appID string, config map[string]interface{}) (*InstallTask, error) {
	if is.catalog == nil {
		return nil, fmt.Errorf("app catalog not loaded")
	}

	appDef := is.catalog.GetApp(appID)
	if appDef == nil {
		return nil, fmt.Errorf("app not found: %s", appID)
	}

	name, _ := config["app_name"].(string)
	if name == "" {
		name = appID
	}

	taskID := fmt.Sprintf("%s-%d", name, time.Now().UnixMilli())
	task := &InstallTask{
		ID:        taskID,
		AppID:     appID,
		Name:      name,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	is.tasksMu.Lock()
	is.tasks[taskID] = task
	is.tasksMu.Unlock()

	go func() {
		is.tasksMu.Lock()
		task.Status = "running"
		is.tasksMu.Unlock()

		// 使用流式输出的回调，实时累积 task.Output
		onOutput := func(line string) {
			is.tasksMu.Lock()
			task.Output += line + "\n"
			is.tasksMu.Unlock()
		}

		app, output, err := is.InstallStreaming(appID, config, onOutput)

		is.tasksMu.Lock()
		task.Output = output
		task.DoneAt = time.Now()
		if err != nil {
			task.Status = "failed"
			task.Error = err.Error()
		} else {
			task.Status = "success"
			_ = app // used for side effect
		}
		is.tasksMu.Unlock()
	}()

	return task, nil
}

// GetTask 获取安装任务状态
func (is *InstalledService) GetTask(taskID string) *InstallTask {
	is.tasksMu.RLock()
	defer is.tasksMu.RUnlock()
	return is.tasks[taskID]
}

// ==================== 生命周期管理 ====================

// Uninstall 卸载应用
func (is *InstalledService) Uninstall(name string) error {
	app := is.Get(name)
	if app == nil {
		return fmt.Errorf("应用未找到: %s", name)
	}

	// docker compose down -v
	cmd := composeCommand("-f", app.ComposePath, "down", "-v")
	if output, err := cmd.CombinedOutput(); err != nil {
		is.logger.Warn("docker compose down failed", zap.String("output", string(output)), zap.Error(err))
	}

	// 删除应用目录
	appDir := filepath.Dir(app.ComposePath)
	os.RemoveAll(appDir)

	// 在锁内删除并保存
	is.mu.Lock()
	delete(is.apps, name)
	err := is.saveLocked()
	is.mu.Unlock()

	return err
}

// Start 启动应用
func (is *InstalledService) Start(name string) error {
	app := is.Get(name)
	if app == nil {
		return fmt.Errorf("应用未找到: %s", name)
	}

	cmd := composeCommand("-f", app.ComposePath, "up", "-d")
	cmd.Dir = filepath.Dir(app.ComposePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("启动失败: %s", string(output))
	}
	return nil
}

// Stop 停止应用
func (is *InstalledService) Stop(name string) error {
	app := is.Get(name)
	if app == nil {
		return fmt.Errorf("应用未找到: %s", name)
	}

	cmd := composeCommand("-f", app.ComposePath, "stop")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("停止失败: %s", string(output))
	}
	return nil
}

// Restart 重启应用
func (is *InstalledService) Restart(name string) error {
	app := is.Get(name)
	if app == nil {
		return fmt.Errorf("应用未找到: %s", name)
	}

	cmd := composeCommand("-f", app.ComposePath, "restart")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("重启失败: %s", string(output))
	}
	return nil
}

// GetLogs 获取应用日志
func (is *InstalledService) GetLogs(name string, tail int) (string, error) {
	app := is.Get(name)
	if app == nil {
		return "", fmt.Errorf("应用未找到: %s", name)
	}

	cmd := composeCommand("-f", app.ComposePath, "logs", "--tail", fmt.Sprintf("%d", tail))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取日志失败: %w", err)
	}
	return string(output), nil
}
