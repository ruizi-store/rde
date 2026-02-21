// Package flatpak 核心服务
package flatpak

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ruizi-store/rde/backend/core/i18n"
	"go.uber.org/zap"
)

// Service Flatpak 服务
type Service struct {
	logger    *zap.Logger
	dataDir   string
	installer *Installer
	desktop   *Desktop

	// Flatpak 缓存
	mu             sync.RWMutex
	installedCache []*FlatpakApp
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, dataDir string) *Service {
	os.MkdirAll(dataDir, 0755)

	installer := NewInstaller(logger)
	desktop := NewDesktop(logger, dataDir, installer)

	s := &Service{
		logger:    logger,
		dataDir:   dataDir,
		installer: installer,
		desktop:   desktop,
	}

	s.loadConfig()
	return s
}

// Start 启动服务
func (s *Service) Start() error {
	config := s.desktop.GetConfig()
	if !config.AutoStart {
		return nil
	}

	// 检查环境是否就绪
	status := s.installer.CheckSystemDeps()
	if !status.Ready {
		s.logger.Info("flatpak environment not ready, skipping auto-start")
		return nil
	}

	// 自动启动桌面
	if err := s.desktop.Start(); err != nil {
		s.logger.Warn("failed to auto-start desktop", zap.Error(err))
		// 不返回错误，auto-start 失败不影响模块启动
	}

	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.desktop.Stop()
}

// ==================== Setup ====================

// GetSetupStatus 获取环境检测状态
func (s *Service) GetSetupStatus() *SetupStatus {
	return s.installer.CheckSystemDeps()
}

// RunSetup 执行环境安装（SSE 流式输出）
func (s *Service) RunSetup(onProgress func(line string), onComplete func(err error)) {
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
	if err := s.desktop.Start(); err != nil {
		onComplete(fmt.Errorf("start desktop: %w", err))
		return
	}

	onProgress("✓ 环境配置完成！")
	onComplete(nil)
}

// ==================== Desktop ====================

// GetDesktopStatus 获取桌面状态
func (s *Service) GetDesktopStatus() *DesktopStatus {
	return s.desktop.GetStatus()
}

// StartDesktop 启动桌面
func (s *Service) StartDesktop() error {
	return s.desktop.Start()
}

// StopDesktop 停止桌面
func (s *Service) StopDesktop() {
	s.desktop.Stop()
}

// RestartDesktop 重启桌面
func (s *Service) RestartDesktop() error {
	return s.desktop.Restart()
}

// GetDesktopConfig 获取桌面配置
func (s *Service) GetDesktopConfig() DesktopConfig {
	return s.desktop.GetConfig()
}

// UpdateDesktopConfig 更新桌面配置
func (s *Service) UpdateDesktopConfig(config DesktopConfig) {
	s.desktop.SetConfig(config)
	s.saveConfig()
}

// ==================== Flatpak 应用管理 ====================

// GetInstalledApps 获取已安装的 Flatpak 应用
func (s *Service) GetInstalledApps() []*FlatpakApp {
	out, err := exec.Command("flatpak", "list", "--app", "--columns=application,name,description,version,size,origin").Output()
	if err != nil {
		s.logger.Warn("failed to list flatpak apps", zap.Error(err))
		return nil
	}

	runningApps := s.desktop.GetRunningApps()
	runningSet := make(map[string]bool)
	for _, app := range runningApps {
		runningSet[app.AppID] = true
	}

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
func (s *Service) InstallApp(appID string, onProgress func(line string), onComplete func(err error)) {
	onProgress(fmt.Sprintf("正在安装 %s ...", appID))

	cmd := exec.Command("flatpak", "install", "-y", "--noninteractive", "flathub", appID)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		onComplete(fmt.Errorf("pipe: %w", err))
		return
	}
	cmd.Stderr = cmd.Stdout // 合并 stderr

	if err := cmd.Start(); err != nil {
		onComplete(fmt.Errorf("start: %w", err))
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		onProgress(scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		onComplete(fmt.Errorf("flatpak install %s: %w", appID, err))
		return
	}

	onProgress(fmt.Sprintf("✓ %s 安装完成", appID))
	onComplete(nil)
}

// UninstallApp 卸载 Flatpak 应用
func (s *Service) UninstallApp(appID string) error {
	cmd := exec.Command("flatpak", "uninstall", "-y", "--noninteractive", appID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("flatpak uninstall %s: %s", appID, string(out))
	}
	return nil
}

// RunApp 在桌面中运行应用
func (s *Service) RunApp(appID string, args []string) error {
	// 查找应用名称
	name := appID
	apps := s.GetInstalledApps()
	for _, app := range apps {
		if app.AppID == appID {
			name = app.Name
			break
		}
	}

	return s.desktop.LaunchApp(appID, name, args)
}

// ==================== 音频管理 ====================

// ensurePulseAudio 确保 PulseAudio 和虚拟声卡就绪
func (s *Service) ensurePulseAudio(onProgress func(line string)) error {
	// 检查是否已运行
	if exec.Command("pulseaudio", "--check").Run() != nil {
		// 尝试 systemd 启动
		onProgress("启动 PulseAudio...")
		if exec.Command("systemctl", "start", "pulseaudio-system.service").Run() != nil {
			// 直接启动 system mode
			if err := exec.Command("pulseaudio", "--system", "--daemonize=yes", "--disallow-exit").Run(); err != nil {
				return fmt.Errorf("start pulseaudio: %w", err)
			}
		}
	}
	onProgress("PulseAudio 已运行")

	// 创建虚拟声卡
	return s.ensureVirtualAudioSink(onProgress)
}

// ensureVirtualAudioSink 创建虚拟声卡
func (s *Service) ensureVirtualAudioSink(onProgress func(line string)) error {
	paEnv := append(os.Environ(), "PULSE_SERVER=unix:/run/pulse/native")

	// 检查是否已存在
	checkCmd := exec.Command("pactl", "list", "sinks", "short")
	checkCmd.Env = paEnv
	out, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("pactl list sinks: %w", err)
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
		exec.Command("apt-get", "update", "-q").Run()
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.Command("apt-get", args...)
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
	var config DesktopConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}
	s.desktop.SetConfig(config)
}

// saveConfig 保存配置
func (s *Service) saveConfig() {
	config := s.desktop.GetConfig()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(s.dataDir, "config.json"), data, 0644)
}

// ==================== 推荐应用列表 ====================

func getRecommendedList() []RecommendedApp {
	return []RecommendedApp{
		// 浏览器
		{AppID: "org.mozilla.firefox", Name: "Firefox", Description: "Mozilla Firefox 浏览器", Category: "browser", Icon: "firefox"},
		{AppID: "com.google.Chrome", Name: "Google Chrome", Description: "Google Chrome 浏览器", Category: "browser", Icon: "chrome"},
		{AppID: "org.chromium.Chromium", Name: "Chromium", Description: "开源 Chromium 浏览器", Category: "browser", Icon: "chromium"},

		// 办公
		{AppID: "org.libreoffice.LibreOffice", Name: "LibreOffice", Description: "开源办公套件", Category: "office", Icon: "libreoffice"},
		{AppID: "org.onlyoffice.desktopeditors", Name: "ONLYOFFICE", Description: "ONLYOFFICE 桌面编辑器", Category: "office", Icon: "onlyoffice"},
		{AppID: "md.obsidian.Obsidian", Name: "Obsidian", Description: "知识管理与笔记", Category: "office", Icon: "obsidian"},

		// 开发
		{AppID: "com.visualstudio.code", Name: "VS Code", Description: "Visual Studio Code 代码编辑器", Category: "development", Icon: "vscode"},
		{AppID: "com.sublimetext.three", Name: "Sublime Text", Description: "高效文本编辑器", Category: "development", Icon: "sublime"},

		// 图形
		{AppID: "org.gimp.GIMP", Name: "GIMP", Description: "GNU 图像处理程序", Category: "graphics", Icon: "gimp"},
		{AppID: "org.inkscape.Inkscape", Name: "Inkscape", Description: "矢量图形编辑器", Category: "graphics", Icon: "inkscape"},
		{AppID: "org.kde.krita", Name: "Krita", Description: "数字绘画应用", Category: "graphics", Icon: "krita"},

		// 多媒体
		{AppID: "org.videolan.VLC", Name: "VLC", Description: "多媒体播放器", Category: "multimedia", Icon: "vlc"},
		{AppID: "org.audacityteam.Audacity", Name: "Audacity", Description: "音频编辑器", Category: "multimedia", Icon: "audacity"},

		// 通讯
		{AppID: "com.discordapp.Discord", Name: "Discord", Description: "语音通讯", Category: "communication", Icon: "discord"},
		{AppID: "org.telegram.desktop", Name: "Telegram", Description: "即时通讯", Category: "communication", Icon: "telegram"},

		// 工具
		{AppID: "org.filezillaproject.Filezilla", Name: "FileZilla", Description: "FTP/SFTP 客户端", Category: "utility", Icon: "filezilla"},
		{AppID: "org.keepassxc.KeePassXC", Name: "KeePassXC", Description: "密码管理器", Category: "utility", Icon: "keepassxc"},
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
