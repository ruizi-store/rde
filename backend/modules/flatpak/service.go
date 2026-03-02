// Package flatpak 核心服务
package flatpak

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ruizi-store/rde/backend/core/i18n"
	"go.uber.org/zap"
)

// installTask 安装任务内部状态
type installTask struct {
	appID     string
	appName   string
	startedAt time.Time
	status    string // installing, success, error
	errMsg    string
	logs      []string      // 缓冲的日志
	subs      []chan string // 订阅者通道
}

// multiUserConfig 多用户配置持久化结构
type multiUserConfig struct {
	Users      map[string]*DesktopConfig `json:"users"`
	NextOffset int                       `json:"next_offset"`
}

// Service Flatpak 服务
type Service struct {
	logger    *zap.Logger
	dataDir   string
	installer *Installer

	// 多用户桌面管理
	desktopMu      sync.RWMutex
	desktops       map[string]*Desktop       // username -> Desktop
	desktopConfigs map[string]*DesktopConfig // username -> config
	nextOffset     int                       // 下一个可用的 display/port 偏移

	// Flatpak 缓存
	mu             sync.RWMutex
	installedCache []*FlatpakApp

	// 安装任务跟踪
	installMu      sync.Mutex
	activeInstalls map[string]*installTask
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, dataDir string) *Service {
	os.MkdirAll(dataDir, 0755)

	installer := NewInstaller(logger)

	s := &Service{
		logger:         logger,
		dataDir:        dataDir,
		installer:      installer,
		desktops:       make(map[string]*Desktop),
		desktopConfigs: make(map[string]*DesktopConfig),
		activeInstalls: make(map[string]*installTask),
	}

	s.loadConfig()
	return s
}

// Start 启动服务
func (s *Service) Start() error {
	// 检查环境是否就绪
	status := s.installer.CheckSystemDeps()
	if !status.Ready {
		s.logger.Info("flatpak environment not ready, skipping auto-start")
		return nil
	}

	// 自动启动所有配置了 auto_start 的用户桌面
	s.desktopMu.Lock()
	defer s.desktopMu.Unlock()

	for username, cfg := range s.desktopConfigs {
		if !cfg.AutoStart {
			continue
		}
		desktop, ok := s.desktops[username]
		if !ok {
			userDataDir := filepath.Join(s.dataDir, "desktops", username)
			desktop = NewDesktop(s.logger, userDataDir, s.installer)
			desktop.SetConfig(*cfg)
			s.desktops[username] = desktop
		}
		if err := desktop.Start(); err != nil {
			s.logger.Warn("failed to auto-start desktop",
				zap.String("username", username), zap.Error(err))
		}
	}

	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.desktopMu.RLock()
	defer s.desktopMu.RUnlock()
	for _, desktop := range s.desktops {
		desktop.Stop()
	}
}

// ==================== Setup ====================

// GetSetupStatus 获取环境检测状态
func (s *Service) GetSetupStatus() *SetupStatus {
	return s.installer.CheckSystemDeps()
}

// RunSetup 执行环境安装（SSE 流式输出）
func (s *Service) RunSetup(username string, onProgress func(line string), onComplete func(err error)) {
	status := s.installer.CheckSystemDeps()

	// 1. 安装系统包
	var missingPkgs []string
	if !status.FlatpakInstalled {
		missingPkgs = append(missingPkgs, "flatpak")
	}
	if !status.OpenboxInstalled {
		missingPkgs = append(missingPkgs, "openbox")
	}
	if !status.PulseAudioInstalled {
		missingPkgs = append(missingPkgs, "pulseaudio", "pulseaudio-utils")
	}
	// 确保 xclip 可用（剪贴板功能需要）
	if _, err := exec.LookPath("xclip"); err != nil {
		missingPkgs = append(missingPkgs, "xclip")
	}

	if len(missingPkgs) > 0 {
		onProgress(fmt.Sprintf("安装系统依赖: %s", strings.Join(missingPkgs, ", ")))
		if err := s.installSystemPackages(missingPkgs, onProgress); err != nil {
			onComplete(fmt.Errorf("install system packages: %w", err))
			return
		}
	}

	// 2. 下载安装 KasmVNC
	if !status.KasmVNCInstalled || s.installer.NeedsUpdate() {
		if err := s.installer.Install(onProgress); err != nil {
			onComplete(fmt.Errorf("install KasmVNC: %w", err))
			return
		}
	} else {
		onProgress("KasmVNC 已安装")
	}

	// 3. 启动 PulseAudio + 虚拟声卡
	onProgress("配置音频...")
	if err := s.ensurePulseAudio(onProgress); err != nil {
		s.logger.Warn("pulseaudio setup failed", zap.Error(err))
		onProgress("⚠ 音频配置失败（非致命）: " + err.Error())
	}

	// 4. 初始化 Flatpak
	onProgress("配置 Flatpak...")
	s.initFlatpak(onProgress)

	// 5. 启动桌面
	onProgress("启动 KasmVNC 桌面...")
	if err := s.StartDesktop(username); err != nil {
		// 将 KasmVNC 的详细错误输出也通过进度回调发送给前端
		errMsg := err.Error()
		if strings.Contains(errMsg, "KasmVNC output:") {
			parts := strings.SplitN(errMsg, "\nKasmVNC output:\n", 2)
			onProgress("✗ " + parts[0])
			if len(parts) > 1 {
				for _, line := range strings.Split(parts[1], "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						onProgress("  | " + line)
					}
				}
			}
		} else {
			onProgress("✗ " + errMsg)
		}
		onComplete(fmt.Errorf("start desktop: %w", err))
		return
	}

	onProgress("✓ 环境配置完成！")
	onComplete(nil)
}

// ==================== Desktop ====================

// getOrCreateDesktop 获取或创建用户桌面实例
func (s *Service) getOrCreateDesktop(username string) *Desktop {
	s.desktopMu.RLock()
	if d, ok := s.desktops[username]; ok {
		s.desktopMu.RUnlock()
		return d
	}
	s.desktopMu.RUnlock()

	s.desktopMu.Lock()
	defer s.desktopMu.Unlock()

	// double-check
	if d, ok := s.desktops[username]; ok {
		return d
	}

	// 分配 display/port
	config := s.allocateConfig(username)

	userDataDir := filepath.Join(s.dataDir, "desktops", username)
	desktop := NewDesktop(s.logger, userDataDir, s.installer)
	desktop.SetConfig(config)
	s.desktops[username] = desktop
	s.saveConfigLocked()

	return desktop
}

// allocateConfig 为用户分配桌面配置（调用方须持有 desktopMu 写锁）
func (s *Service) allocateConfig(username string) DesktopConfig {
	if cfg, ok := s.desktopConfigs[username]; ok {
		return *cfg
	}

	offset := s.nextOffset
	s.nextOffset++

	cfg := DesktopConfig{
		Display:           100 + offset,
		WebSocketPort:     6100 + offset,
		DefaultResolution: "1920x1080",
		AudioEnabled:      true,
		ClipboardSync:     true,
		AutoStart:         true,
	}
	s.desktopConfigs[username] = &cfg
	return cfg
}

// GetDesktopStatus 获取桌面状态
func (s *Service) GetDesktopStatus(username string) *DesktopStatus {
	desktop := s.getOrCreateDesktop(username)
	return desktop.GetStatus()
}

// StartDesktop 启动桌面
func (s *Service) StartDesktop(username string) error {
	desktop := s.getOrCreateDesktop(username)
	return desktop.Start()
}

// StopDesktop 停止桌面
func (s *Service) StopDesktop(username string) {
	s.desktopMu.RLock()
	desktop, ok := s.desktops[username]
	s.desktopMu.RUnlock()
	if ok {
		desktop.Stop()
	}
}

// RestartDesktop 重启桌面
func (s *Service) RestartDesktop(username string) error {
	desktop := s.getOrCreateDesktop(username)
	return desktop.Restart()
}

// GetDesktopConfig 获取桌面配置
func (s *Service) GetDesktopConfig(username string) DesktopConfig {
	desktop := s.getOrCreateDesktop(username)
	return desktop.GetConfig()
}

// UpdateDesktopConfig 更新桌面配置
func (s *Service) UpdateDesktopConfig(username string, config DesktopConfig) {
	desktop := s.getOrCreateDesktop(username)
	desktop.SetConfig(config)
	s.desktopMu.Lock()
	s.desktopConfigs[username] = &config
	s.saveConfigLocked()
	s.desktopMu.Unlock()
}

// ==================== Clipboard ====================

// SetClipboard 设置用户桌面剪贴板内容
func (s *Service) SetClipboard(username, text string) error {
	desktop := s.getOrCreateDesktop(username)
	if !desktop.IsRunning() {
		return fmt.Errorf("desktop not running")
	}
	display := desktop.GetConfig().Display

	cmd := exec.Command("xclip", "-display", fmt.Sprintf(":%d", display), "-selection", "clipboard")
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xclip set: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// GetClipboard 获取用户桌面剪贴板内容
func (s *Service) GetClipboard(username string) (string, error) {
	desktop := s.getOrCreateDesktop(username)
	if !desktop.IsRunning() {
		return "", fmt.Errorf("desktop not running")
	}
	display := desktop.GetConfig().Display

	out, err := exec.Command("xclip", "-display", fmt.Sprintf(":%d", display), "-selection", "clipboard", "-o").Output()
	if err != nil {
		// xclip returns error when clipboard is empty, return empty string
		return "", nil
	}
	return string(out), nil
}

// ==================== Flatpak 应用管理 ====================

// GetInstalledApps 获取已安装的 Flatpak 应用
func (s *Service) GetInstalledApps() []*FlatpakApp {
	out, err := exec.Command("flatpak", "list", "--app", "--columns=application,name,description,version,size,origin").Output()
	if err != nil {
		s.logger.Warn("failed to list flatpak apps", zap.Error(err))
		return nil
	}

	// 从所有用户桌面汇总运行中的应用
	runningSet := make(map[string]bool)
	s.desktopMu.RLock()
	for _, desktop := range s.desktops {
		for _, app := range desktop.GetRunningApps() {
			runningSet[app.AppID] = true
		}
	}
	s.desktopMu.RUnlock()

	var apps []*FlatpakApp
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		app := &FlatpakApp{
			AppID:     fields[0],
			Name:      fields[1],
			Installed: true,
			Running:   runningSet[fields[0]],
		}
		if len(fields) > 2 {
			app.Description = fields[2]
		}
		if len(fields) > 3 {
			app.Version = fields[3]
		}
		if len(fields) > 4 {
			app.Size = fields[4]
		}
		if len(fields) > 5 {
			app.Remote = fields[5]
		}

		// 获取图标 URL
		app.Icon = s.getAppIconURL(app.AppID)

		apps = append(apps, app)
	}

	return apps
}

// SearchApps 搜索 Flathub 应用
func (s *Service) SearchApps(query string, limit int) []*FlatpakApp {
	if limit <= 0 {
		limit = 50
	}

	out, err := exec.Command("flatpak", "search", query, "--columns=application,name,description,version,remotes").Output()
	if err != nil {
		s.logger.Warn("flatpak search failed", zap.Error(err))
		return nil
	}

	// 获取已安装列表
	installed := s.getInstalledSet()

	var apps []*FlatpakApp
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		app := &FlatpakApp{
			AppID:     fields[0],
			Name:      fields[1],
			Installed: installed[fields[0]],
		}
		if len(fields) > 2 {
			app.Description = fields[2]
		}
		if len(fields) > 3 {
			app.Version = fields[3]
		}
		if len(fields) > 4 {
			app.Remote = fields[4]
		}

		apps = append(apps, app)

		if len(apps) >= limit {
			break
		}
	}

	return apps
}

// GetRecommendedApps 获取推荐应用列表
func (s *Service) GetRecommendedApps(category string) []*FlatpakApp {
	installed := s.getInstalledSet()

	recommended := getRecommendedList()

	var apps []*FlatpakApp
	for _, rec := range recommended {
		if category != "" && rec.Category != category {
			continue
		}
		app := &FlatpakApp{
			AppID:       rec.AppID,
			Name:        rec.Name,
			Description: rec.Description,
			Category:    rec.Category,
			Icon:        rec.Icon,
			Installed:   installed[rec.AppID],
		}
		apps = append(apps, app)
	}

	return apps
}

// InstallApp 安装 Flatpak 应用（带流式进度）
// 如果已有同一应用的安装任务正在运行，直接返回（前端应通过 WatchInstallProgress 重连）
func (s *Service) InstallApp(appID string, appName string, onProgress func(line string), onComplete func(err error)) {
	s.installMu.Lock()
	if task, exists := s.activeInstalls[appID]; exists && task.status == "installing" {
		s.installMu.Unlock()
		onComplete(fmt.Errorf("already installing"))
		return
	}

	task := &installTask{
		appID:     appID,
		appName:   appName,
		startedAt: time.Now(),
		status:    "installing",
		logs:      []string{},
	}
	s.activeInstalls[appID] = task
	s.installMu.Unlock()

	// 广播日志到缓冲和所有订阅者
	broadcast := func(line string) {
		s.installMu.Lock()
		task.logs = append(task.logs, line)
		subs := make([]chan string, len(task.subs))
		copy(subs, task.subs)
		s.installMu.Unlock()
		for _, ch := range subs {
			select {
			case ch <- line:
			default: // 订阅者消费太慢则丢弃
			}
		}
	}

	broadcast(fmt.Sprintf("正在安装 %s ...", appID))
	onProgress(fmt.Sprintf("正在安装 %s ...", appID))

	cmd := exec.Command("flatpak", "install", "-y", "--noninteractive", "flathub", appID)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.finishInstall(appID, fmt.Errorf("pipe: %w", err))
		onComplete(fmt.Errorf("pipe: %w", err))
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		s.finishInstall(appID, fmt.Errorf("start: %w", err))
		onComplete(fmt.Errorf("start: %w", err))
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		broadcast(line)
		onProgress(line)
	}

	if err := cmd.Wait(); err != nil {
		installErr := fmt.Errorf("flatpak install %s: %w", appID, err)
		broadcast("✗ 安装失败: " + err.Error())
		s.finishInstall(appID, installErr)
		onComplete(installErr)
		return
	}

	msg := fmt.Sprintf("✓ %s 安装完成", appID)
	broadcast(msg)
	onProgress(msg)
	s.finishInstall(appID, nil)
	onComplete(nil)
}

// finishInstall 完成安装任务，关闭所有订阅者
func (s *Service) finishInstall(appID string, err error) {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	task, exists := s.activeInstalls[appID]
	if !exists {
		return
	}
	if err != nil {
		task.status = "error"
		task.errMsg = err.Error()
	} else {
		task.status = "success"
	}
	// 关闭所有订阅者通道
	for _, ch := range task.subs {
		close(ch)
	}
	task.subs = nil
	// 30秒后清理已完成的任务
	go func() {
		time.Sleep(30 * time.Second)
		s.installMu.Lock()
		if t, ok := s.activeInstalls[appID]; ok && t.status != "installing" {
			delete(s.activeInstalls, appID)
		}
		s.installMu.Unlock()
	}()
}

// GetActiveInstalls 获取当前正在安装的应用列表
func (s *Service) GetActiveInstalls() []ActiveInstall {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	var result []ActiveInstall
	for _, task := range s.activeInstalls {
		result = append(result, ActiveInstall{
			AppID:     task.appID,
			AppName:   task.appName,
			StartedAt: task.startedAt,
			Status:    task.status,
			Error:     task.errMsg,
		})
	}
	return result
}

// WatchInstallProgress 订阅安装进度，返回 (历史日志, 实时通道, 任务状态, 是否存在)
func (s *Service) WatchInstallProgress(appID string) (logs []string, ch chan string, status string, errMsg string, exists bool) {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	task, ok := s.activeInstalls[appID]
	if !ok {
		return nil, nil, "", "", false
	}
	// 拷贝历史日志
	logs = make([]string, len(task.logs))
	copy(logs, task.logs)
	// 如果还在安装中，创建订阅通道
	if task.status == "installing" {
		ch = make(chan string, 64)
		task.subs = append(task.subs, ch)
	}
	return logs, ch, task.status, task.errMsg, true
}

// UnsubscribeInstall 取消订阅
func (s *Service) UnsubscribeInstall(appID string, ch chan string) {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	task, ok := s.activeInstalls[appID]
	if !ok {
		return
	}
	for i, sub := range task.subs {
		if sub == ch {
			task.subs = append(task.subs[:i], task.subs[i+1:]...)
			break
		}
	}
}

// UninstallApp 卸载 Flatpak 应用
func (s *Service) UninstallApp(appID string) error {
	cmd := exec.Command("flatpak", "uninstall", "-y", "--noninteractive", appID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("flatpak uninstall %s: %s", appID, string(out))
	}
	return nil
}

// RunApp 在桌面中运行应用（以指定用户身份）
func (s *Service) RunApp(appID string, args []string, username string) error {
	// 查找系统用户信息
	sysUser, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("lookup user %s: %w", username, err)
	}
	uid, _ := strconv.Atoi(sysUser.Uid)
	gid, _ := strconv.Atoi(sysUser.Gid)

	// 确保用户 runtime 目录存在
	if err := s.ensureUserRuntime(username, uid, gid); err != nil {
		s.logger.Warn("ensure user runtime failed", zap.Error(err))
		// 不阻断，继续尝试运行
	}

	// 查找应用名称
	name := appID
	apps := s.GetInstalledApps()
	for _, app := range apps {
		if app.AppID == appID {
			name = app.Name
			break
		}
	}

	// 使用用户自己的桌面实例
	desktop := s.getOrCreateDesktop(username)
	return desktop.LaunchApp(appID, name, args, username, uid)
}

// ensureUserRuntime 确保用户 XDG_RUNTIME_DIR 和 dbus 会话总线存在
func (s *Service) ensureUserRuntime(username string, uid, gid int) error {
	runtimeDir := fmt.Sprintf("/run/user/%d", uid)
	busPath := filepath.Join(runtimeDir, "bus")

	// 检查目录是否存在
	if _, err := os.Stat(runtimeDir); os.IsNotExist(err) {
		// 启用 linger（允许用户服务持续运行）
		exec.Command("loginctl", "enable-linger", username).Run()

		// 创建 runtime 目录
		if err := os.MkdirAll(runtimeDir, 0700); err != nil {
			return fmt.Errorf("create runtime dir: %w", err)
		}

		// 设置所有者
		if err := os.Chown(runtimeDir, uid, gid); err != nil {
			return fmt.Errorf("chown runtime dir: %w", err)
		}
	}

	// 检查 dbus 会话总线是否存在
	if _, err := os.Stat(busPath); os.IsNotExist(err) {
		// 以目标用户身份启动 dbus-daemon
		cmd := exec.Command("runuser", "-u", username, "--",
			"dbus-daemon", "--session",
			fmt.Sprintf("--address=unix:path=%s", busPath),
			"--fork", "--print-pid")
		cmd.Env = append(os.Environ(), fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir))
		if output, err := cmd.CombinedOutput(); err != nil {
			s.logger.Warn("start user dbus failed",
				zap.String("username", username),
				zap.String("output", string(output)),
				zap.Error(err))
		} else {
			s.logger.Info("started user dbus", zap.String("username", username))
		}
	}
	return nil
}

// ==================== 音频管理 ====================

// ensurePulseAudio 确保 PulseAudio 和虚拟声卡就绪
func (s *Service) ensurePulseAudio(onProgress func(line string)) error {
	// 检查是否已运行
	if exec.Command("pulseaudio", "--check").Run() != nil {
		onProgress("启动 PulseAudio...")

		// 确保 /run/pulse 目录存在
		os.MkdirAll("/run/pulse", 0755)

		// 尝试启动 PulseAudio（按优先级依次尝试）
		started := false

		// 方式1：用户模式 daemon
		if !started {
			cmd := exec.Command("pulseaudio", "--daemonize=yes", "--exit-idle-time=-1")
			if err := cmd.Run(); err != nil {
				onProgress(fmt.Sprintf("用户模式启动失败: %v", err))
			} else {
				started = true
			}
		}

		// 方式2：system mode
		if !started {
			onProgress("尝试 system mode...")
			cmd := exec.Command("pulseaudio", "--system", "--daemonize=yes", "--disallow-exit")
			if err := cmd.Run(); err != nil {
				onProgress(fmt.Sprintf("system mode 启动失败: %v", err))
			} else {
				started = true
			}
		}

		if !started {
			return fmt.Errorf("无法启动 PulseAudio")
		}

		// 等待 PulseAudio 就绪
		time.Sleep(500 * time.Millisecond)
	}
	onProgress("PulseAudio 已运行")

	// 设置 unix socket 用于 Flatpak 应用访问
	s.ensurePulseSocket(onProgress)

	// 创建虚拟声卡
	return s.ensureVirtualAudioSink(onProgress)
}

// ensurePulseSocket 确保 PulseAudio 的 unix socket 存在
func (s *Service) ensurePulseSocket(onProgress func(line string)) {
	// 检查 /run/pulse/native 是否存在
	if _, err := os.Stat("/run/pulse/native"); err == nil {
		return
	}

	// 加载 module-native-protocol-unix 使得 Flatpak 应用能通过 socket 连接
	cmd := exec.Command("pactl", "load-module", "module-native-protocol-unix",
		"auth-anonymous=1", "socket=/run/pulse/native")
	if out, err := cmd.CombinedOutput(); err != nil {
		onProgress(fmt.Sprintf("⚠ 加载 unix socket 模块失败: %v (%s)", err, strings.TrimSpace(string(out))))
	}
}

// ensureVirtualAudioSink 创建虚拟声卡
func (s *Service) ensureVirtualAudioSink(onProgress func(line string)) error {
	// 尝试多种连接方式
	var paEnv []string
	var out []byte
	var err error

	// 先用默认连接试
	checkCmd := exec.Command("pactl", "list", "sinks", "short")
	out, err = checkCmd.Output()
	if err != nil {
		// 用 unix socket 试
		paEnv = append(os.Environ(), "PULSE_SERVER=unix:/run/pulse/native")
		checkCmd2 := exec.Command("pactl", "list", "sinks", "short")
		checkCmd2.Env = paEnv
		out, err = checkCmd2.Output()
		if err != nil {
			return fmt.Errorf("pactl list sinks: %w", err)
		}
	}
	if strings.Contains(string(out), "virtual_speaker") {
		// 确保设为默认
		setDefault := exec.Command("pactl", "set-default-sink", "virtual_speaker")
		setDefault.Env = paEnv
		setDefault.Run()
		setDefaultSrc := exec.Command("pactl", "set-default-source", "virtual_speaker.monitor")
		setDefaultSrc.Env = paEnv
		setDefaultSrc.Run()
		onProgress("虚拟声卡已就绪")
		return nil
	}

	// 创建虚拟声卡
	onProgress("创建虚拟声卡 (module-null-sink)...")
	loadCmd := exec.Command("pactl", "load-module", "module-null-sink",
		"sink_name=virtual_speaker",
		`sink_properties=device.description="Virtual_Speaker"`)
	loadCmd.Env = paEnv
	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("load module-null-sink: %w", err)
	}

	// 设为默认
	setDefault := exec.Command("pactl", "set-default-sink", "virtual_speaker")
	setDefault.Env = paEnv
	setDefault.Run()

	setDefaultSrc := exec.Command("pactl", "set-default-source", "virtual_speaker.monitor")
	setDefaultSrc.Env = paEnv
	setDefaultSrc.Run()

	onProgress("✓ 虚拟声卡创建完成")
	return nil
}

// ==================== Flatpak 初始化 ====================

// initFlatpak 初始化 Flatpak（添加 Flathub remote）
func (s *Service) initFlatpak(onProgress func(line string)) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		onProgress("⚠ flatpak 未安装")
		return
	}

	// 检查是否已添加 flathub
	out, err := exec.Command("flatpak", "remotes", "--columns=name").Output()
	if err != nil {
		onProgress("⚠ 无法检查 flatpak remotes")
		return
	}

	hasFlathub := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "flathub" {
			hasFlathub = true
			break
		}
	}

	if !hasFlathub {
		// 获取镜像地址
		flatpakMirror := i18n.GetMirrorField("flatpak", "repo")
		if flatpakMirror == "" {
			flatpakMirror = "https://flathub.org/repo"
		}
		onProgress(fmt.Sprintf("添加 Flathub remote（镜像: %s）...", flatpakMirror))
		cmd := exec.Command("flatpak", "remote-add", "--if-not-exists", "flathub",
			flatpakMirror+"/flathub.flatpakrepo")
		if err := cmd.Run(); err != nil {
			onProgress("⚠ 使用镜像失败，尝试官方源...")
			cmd = exec.Command("flatpak", "remote-add", "--if-not-exists", "flathub",
				"https://flathub.org/repo/flathub.flatpakrepo")
			cmd.Run()
		}
	} else {
		// 配置镜像
		flatpakURL := i18n.GetMirrorField("flatpak", "url")
		if flatpakURL != "" {
			exec.Command("flatpak", "remote-modify", "--url="+flatpakURL, "flathub").Run()
		}
		onProgress("Flathub 已配置")
	}
}

// ==================== 系统包安装 ====================

// installSystemPackages 安装系统包（自动检测包管理器）
func (s *Service) installSystemPackages(packages []string, onProgress func(line string)) error {
	// 检测包管理器
	var cmd *exec.Cmd
	if _, err := exec.LookPath("apt-get"); err == nil {
		// Debian/Ubuntu
		onProgress("检测到 apt 包管理器")
		// 先修复可能存在的待配置包
		fixCmd := exec.Command("dpkg", "--configure", "--force-confold", "-a")
		fixCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		fixCmd.Run()
		exec.Command("apt-get", "update", "-q").Run()
		args := append([]string{"install", "-y", "-o", "Dpkg::Options::=--force-confold"}, packages...)
		cmd = exec.Command("apt-get", args...)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	} else if _, err := exec.LookPath("pacman"); err == nil {
		// Arch Linux
		onProgress("检测到 pacman 包管理器")
		args := append([]string{"-S", "--noconfirm", "--needed"}, packages...)
		cmd = exec.Command("pacman", args...)
	} else if _, err := exec.LookPath("dnf"); err == nil {
		// Fedora/RHEL
		onProgress("检测到 dnf 包管理器")
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.Command("dnf", args...)
	} else {
		return fmt.Errorf("no supported package manager found (need apt/pacman/dnf)")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		onProgress(scanner.Text())
	}

	return cmd.Wait()
}

// ==================== 工具方法 ====================

// getInstalledSet 获取已安装应用 ID 集合
func (s *Service) getInstalledSet() map[string]bool {
	installed := make(map[string]bool)
	out, err := exec.Command("flatpak", "list", "--app", "--columns=application").Output()
	if err != nil {
		return installed
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		id := strings.TrimSpace(line)
		if id != "" {
			installed[id] = true
		}
	}
	return installed
}

// getAppIconURL 获取应用图标 URL
func (s *Service) getAppIconURL(appID string) string {
	// Flatpak 应用图标路径
	paths := []string{
		filepath.Join("/var/lib/flatpak/exports/share/icons/hicolor/128x128/apps", appID+".png"),
		filepath.Join("/var/lib/flatpak/exports/share/icons/hicolor/64x64/apps", appID+".png"),
		filepath.Join("/var/lib/flatpak/exports/share/icons/hicolor/scalable/apps", appID+".svg"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return "/api/v1/flatpak/icons/" + appID
		}
	}
	return ""
}

// loadConfig 加载配置
func (s *Service) loadConfig() {
	configFile := filepath.Join(s.dataDir, "config.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return
	}

	// 尝试新格式（多用户）
	var cfg multiUserConfig
	if err := json.Unmarshal(data, &cfg); err == nil && cfg.Users != nil {
		s.desktopConfigs = cfg.Users
		s.nextOffset = cfg.NextOffset
		return
	}

	// 兼容旧格式（单用户）：迁移到 "admin" 用户
	var legacy DesktopConfig
	if err := json.Unmarshal(data, &legacy); err == nil && legacy.WebSocketPort > 0 {
		s.desktopConfigs["admin"] = &legacy
		s.nextOffset = 1 // admin 使用 offset 0
		s.logger.Info("migrated legacy single-user config to multi-user")
	}
}

// saveConfig 保存配置（调用方须持有 desktopMu 锁）
func (s *Service) saveConfigLocked() {
	cfg := multiUserConfig{
		Users:      s.desktopConfigs,
		NextOffset: s.nextOffset,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(s.dataDir, "config.json"), data, 0644)
}

// ==================== 推荐应用列表 ====================

// flathubArch 返回 Flathub CDN 使用的架构名
func flathubArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "aarch64"
	case "arm":
		return "armhf"
	default:
		return "x86_64"
	}
}

func flathubIcon(appID string) string {
	return fmt.Sprintf("https://dl.flathub.org/repo/appstream/%s/icons/128x128/%s.png", flathubArch(), appID)
}

func getRecommendedList() []RecommendedApp {
	return []RecommendedApp{
		// 浏览器
		{AppID: "org.mozilla.firefox", Name: "Firefox", Description: "Mozilla Firefox 浏览器", Category: "browser", Icon: flathubIcon("org.mozilla.firefox")},
		{AppID: "com.google.Chrome", Name: "Google Chrome", Description: "Google Chrome 浏览器", Category: "browser", Icon: flathubIcon("com.google.Chrome")},
		{AppID: "org.chromium.Chromium", Name: "Chromium", Description: "开源 Chromium 浏览器", Category: "browser", Icon: flathubIcon("org.chromium.Chromium")},

		// 办公
		{AppID: "org.libreoffice.LibreOffice", Name: "LibreOffice", Description: "开源办公套件", Category: "office", Icon: flathubIcon("org.libreoffice.LibreOffice")},
		{AppID: "org.onlyoffice.desktopeditors", Name: "ONLYOFFICE", Description: "ONLYOFFICE 桌面编辑器", Category: "office", Icon: flathubIcon("org.onlyoffice.desktopeditors")},
		{AppID: "md.obsidian.Obsidian", Name: "Obsidian", Description: "知识管理与笔记", Category: "office", Icon: flathubIcon("md.obsidian.Obsidian")},

		// 开发
		{AppID: "com.visualstudio.code", Name: "VS Code", Description: "Visual Studio Code 代码编辑器", Category: "development", Icon: flathubIcon("com.visualstudio.code")},
		{AppID: "com.sublimetext.three", Name: "Sublime Text", Description: "高效文本编辑器", Category: "development", Icon: flathubIcon("com.sublimetext.three")},

		// 图形
		{AppID: "org.gimp.GIMP", Name: "GIMP", Description: "GNU 图像处理程序", Category: "graphics", Icon: flathubIcon("org.gimp.GIMP")},
		{AppID: "org.inkscape.Inkscape", Name: "Inkscape", Description: "矢量图形编辑器", Category: "graphics", Icon: flathubIcon("org.inkscape.Inkscape")},
		{AppID: "org.kde.krita", Name: "Krita", Description: "数字绘画应用", Category: "graphics", Icon: flathubIcon("org.kde.krita")},

		// 多媒体
		{AppID: "org.videolan.VLC", Name: "VLC", Description: "多媒体播放器", Category: "multimedia", Icon: flathubIcon("org.videolan.VLC")},
		{AppID: "org.audacityteam.Audacity", Name: "Audacity", Description: "音频编辑器", Category: "multimedia", Icon: flathubIcon("org.audacityteam.Audacity")},

		// 通讯
		{AppID: "com.discordapp.Discord", Name: "Discord", Description: "语音通讯", Category: "communication", Icon: flathubIcon("com.discordapp.Discord")},
		{AppID: "org.telegram.desktop", Name: "Telegram", Description: "即时通讯", Category: "communication", Icon: flathubIcon("org.telegram.desktop")},

		// 工具
		{AppID: "org.filezillaproject.Filezilla", Name: "FileZilla", Description: "FTP/SFTP 客户端", Category: "utility", Icon: flathubIcon("org.filezillaproject.Filezilla")},
		{AppID: "org.keepassxc.KeePassXC", Name: "KeePassXC", Description: "密码管理器", Category: "utility", Icon: flathubIcon("org.keepassxc.KeePassXC")},
	}
}

// GetRecommendedCategories 获取推荐分类
func (s *Service) GetRecommendedCategories() []string {
	catSet := make(map[string]bool)
	for _, app := range getRecommendedList() {
		catSet[app.Category] = true
	}
	var cats []string
	for c := range catSet {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	return cats
}
