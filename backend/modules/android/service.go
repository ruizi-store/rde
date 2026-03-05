// Package android Android 投屏服务
package android

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ruizi-store/rde/backend/modules/android/adb"
	"github.com/ruizi-store/rde/backend/modules/android/scrcpy"

	"go.uber.org/zap"
)

// Service Android 投屏服务
type Service struct {
	config ScrcpyConfig
	logger *zap.Logger

	mu             sync.RWMutex
	clients        map[*ClientConn]bool
	sessions       map[string]*Session
	running        bool
	currentSession *Session

	// ADB 客户端
	adbClient *adb.Client

	// scrcpy 客户端
	scrcpyClient *scrcpy.Client
	cancelFn     context.CancelFunc

	// 视频信息
	deviceWidth  int
	deviceHeight int

	// 缓存帧
	lastConfigFrame []byte
	lastKeyFrame    []byte
	lastAudioConfig []byte
}

// NewService 创建服务
func NewService(logger *zap.Logger) *Service {
	return &Service{
		config:   DefaultScrcpyConfig(),
		logger:   logger,
		clients:  make(map[*ClientConn]bool),
		sessions: make(map[string]*Session),
	}
}

// initADBClient 初始化 ADB 客户端
func (s *Service) initADBClient() error {
	if s.adbClient != nil {
		return nil
	}

	// 确保 ADB server 正在运行
	if err := EnsureADBServer(); err != nil {
		s.logger.Warn("Failed to start ADB server", zap.Error(err))
		return fmt.Errorf("adb not available: %w", err)
	}

	client, err := adb.NewClient()
	if err != nil {
		s.logger.Warn("Failed to create ADB client", zap.Error(err))
		return err
	}

	s.adbClient = client
	s.logger.Info("ADB client initialized")
	return nil
}

// autoConnectLocalContainer 自动尝试连接本地 Android 容器
func (s *Service) autoConnectLocalContainer() {
	if !CheckContainerRunning("ruizios-android") {
		return
	}

	devices, _ := s.adbClient.ListDevices()
	for _, d := range devices {
		if d.Serial == "localhost:5555" && d.Connected {
			return
		}
	}

	s.logger.Info("Auto-connecting to local Android container")
	s.adbClient.Connect("localhost:5555")
}

// GetDevices 获取设备列表
func (s *Service) GetDevices() ([]Device, error) {
	if err := s.initADBClient(); err != nil {
		s.logger.Warn("ADB not available", zap.Error(err))
		return []Device{}, nil
	}

	s.autoConnectLocalContainer()

	devices, err := s.adbClient.ListDevices()
	if err != nil {
		s.logger.Warn("Failed to list devices", zap.Error(err))
		return []Device{}, nil
	}

	var result []Device
	for _, d := range devices {
		name := d.Model
		if name == "" {
			name = d.Serial
		}
		status := "offline"
		if d.Connected {
			status = "device"
		}
		result = append(result, Device{
			ID:             d.Serial,
			Name:           name,
			Serial:         d.Serial,
			Model:          d.Model,
			Brand:          d.Brand,
			AndroidVersion: d.AndroidVersion,
			Status:         status,
			Connected:      d.Connected,
		})
	}

	return result, nil
}

// ConnectDevice 连接设备
func (s *Service) ConnectDevice(serial string) error {
	if err := s.initADBClient(); err != nil {
		return err
	}

	err := s.adbClient.Connect(serial)
	if err != nil {
		return fmt.Errorf("adb connect failed: %w", err)
	}

	s.logger.Info("Device connected", zap.String("serial", serial))
	return nil
}

// DisconnectDevice 断开设备
func (s *Service) DisconnectDevice(serial string) error {
	if s.adbClient == nil {
		return nil
	}

	return s.adbClient.Disconnect(serial)
}

// StartSession 启动投屏会话
func (s *Service) StartSession(serial string, config *ScrcpyConfig) (*Session, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		s.logger.Info("Stopping existing session before starting new one")
		if err := s.StopSession(""); err != nil {
			s.logger.Warn("Failed to stop existing session", zap.Error(err))
		}
	} else {
		s.mu.Unlock()
	}

	if err := s.initADBClient(); err != nil {
		return nil, fmt.Errorf("ADB not available: %w", err)
	}

	// 合并配置
	defaults := DefaultScrcpyConfig()
	if config == nil {
		config = &defaults
	} else {
		if config.MaxSize == 0 {
			config.MaxSize = defaults.MaxSize
		}
		if config.Bitrate == 0 {
			config.Bitrate = defaults.Bitrate
		}
		if config.MaxFps == 0 {
			config.MaxFps = defaults.MaxFps
		}
		if config.VideoCodec == "" {
			config.VideoCodec = defaults.VideoCodec
		}
		if config.AudioCodec == "" {
			config.AudioCodec = defaults.AudioCodec
		}
		if config.Orientation == "" {
			config.Orientation = defaults.Orientation
		}
	}

	s.config = *config

	session := &Session{
		ID:           generateID(),
		DeviceSerial: serial,
		StartedAt:    time.Now(),
		Width:        config.MaxSize,
		Height:       config.MaxSize,
		Bitrate:      config.Bitrate,
		MaxFps:       config.MaxFps,
		VideoCodec:   config.VideoCodec,
		AudioEnabled: config.AudioEnabled,
		Status:       "starting",
	}

	// 创建 scrcpy 配置
	scrcpyConfig := scrcpy.Config{
		MaxSize:            config.MaxSize,
		Bitrate:            config.Bitrate,
		MaxFps:             config.MaxFps,
		VideoCodec:         config.VideoCodec,
		AudioCodec:         config.AudioCodec,
		AudioEnabled:       config.AudioEnabled,
		TurnScreenOff:      config.TurnScreen,
		StayAwake:          config.StayAwake,
		ShowTouches:        config.ShowTouches,
		ControlEnabled:     true,
		CaptureOrientation: config.Orientation,
	}

	scrcpyClient, err := scrcpy.NewClient(s.adbClient, serial, scrcpyConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create scrcpy client: %w", err)
	}

	// 设置回调
	scrcpyClient.SetVideoCallback(func(data []byte, pts int64, isKeyFrame bool) {
		s.handleVideoFrame(data, isKeyFrame)
	})

	scrcpyClient.SetAudioCallback(func(data []byte) {
		s.handleAudioFrame(data)
	})

	scrcpyClient.SetAudioConfigCallback(func(data []byte) {
		s.handleAudioConfig(data)
	})

	scrcpyClient.SetErrorCallback(func(err error) {
		s.logger.Error("scrcpy error", zap.Error(err))
	})

	scrcpyClient.SetDisconnectCallback(func() {
		s.handleProcessExit()
	})

	// 启动 scrcpy
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel

	if err := scrcpyClient.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start scrcpy: %w", err)
	}

	s.mu.Lock()
	s.running = true
	s.scrcpyClient = scrcpyClient
	s.currentSession = session
	s.sessions[session.ID] = session
	session.Status = "running"

	width, height := scrcpyClient.ScreenSize()
	if width > 0 && height > 0 {
		session.Width = width
		session.Height = height
		s.deviceWidth = width
		s.deviceHeight = height
	}
	s.mu.Unlock()

	s.logger.Info("Session started",
		zap.String("session_id", session.ID),
		zap.String("device", serial),
		zap.Int("width", session.Width),
		zap.Int("height", session.Height))

	return session, nil
}

// handleVideoFrame 处理视频帧
func (s *Service) handleVideoFrame(data []byte, isKeyFrame bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if isKeyFrame {
		s.lastKeyFrame = data
		if isConfigFrame(data) {
			s.lastConfigFrame = data
		}
	}

	// 构建二进制消息: type(1 byte) + data
	msg := make([]byte, 1+len(data))
	msg[0] = 2 // video type
	copy(msg[1:], data)

	for client := range s.clients {
		if err := client.SendBinary(msg); err != nil {
			s.logger.Debug("Failed to send video to client", zap.Error(err))
		}
	}
}

// handleAudioConfig 处理音频配置包
func (s *Service) handleAudioConfig(data []byte) {
	s.mu.Lock()
	s.lastAudioConfig = make([]byte, len(data))
	copy(s.lastAudioConfig, data)
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	msg := make([]byte, 1+len(data))
	msg[0] = 4 // audio config type
	copy(msg[1:], data)

	for client := range s.clients {
		if err := client.SendBinary(msg); err != nil {
			s.logger.Debug("Failed to send audio config to client", zap.Error(err))
		}
	}
}

// handleAudioFrame 处理音频帧
func (s *Service) handleAudioFrame(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.clients) == 0 {
		return
	}

	msg := make([]byte, 1+len(data))
	msg[0] = 3 // audio type
	copy(msg[1:], data)

	for client := range s.clients {
		if err := client.SendBinary(msg); err != nil {
			s.logger.Debug("Failed to send audio to client", zap.Error(err))
		}
	}
}

// handleProcessExit 处理进程退出
func (s *Service) handleProcessExit() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	if s.currentSession != nil {
		s.currentSession.Status = "stopped"
		s.currentSession = nil
	}

	for client := range s.clients {
		client.Send(WSMessage{
			Type:  MsgTypeError,
			Error: "scrcpy disconnected",
		})
	}

	s.logger.Info("Session stopped")
}

// isConfigFrame 检测是否是配置帧
func isConfigFrame(data []byte) bool {
	if len(data) < 5 {
		return false
	}
	nalType := data[4] & 0x1F
	return nalType == 7 || nalType == 8
}

// StopSession 停止会话
func (s *Service) StopSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancelFn != nil {
		s.cancelFn()
	}

	if s.scrcpyClient != nil {
		s.scrcpyClient.Stop()
		s.scrcpyClient = nil
	}

	s.running = false
	if s.currentSession != nil {
		s.currentSession.Status = "stopped"
		s.currentSession = nil
	}

	return nil
}

// GetCurrentSession 获取当前会话
func (s *Service) GetCurrentSession() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentSession
}

// IsRunning 是否运行中
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// AddClient 添加 WebSocket 客户端
func (s *Service) AddClient(client *ClientConn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[client] = true

	// 发送缓存的配置帧和关键帧
	if s.lastConfigFrame != nil {
		msg := make([]byte, 1+len(s.lastConfigFrame))
		msg[0] = 2
		copy(msg[1:], s.lastConfigFrame)
		client.SendBinary(msg)
	}
	if s.lastKeyFrame != nil {
		msg := make([]byte, 1+len(s.lastKeyFrame))
		msg[0] = 2
		copy(msg[1:], s.lastKeyFrame)
		client.SendBinary(msg)
	}

	// 发送缓存的音频配置
	if s.lastAudioConfig != nil {
		msg := make([]byte, 1+len(s.lastAudioConfig))
		msg[0] = 4
		copy(msg[1:], s.lastAudioConfig)
		client.SendBinary(msg)
	}
}

// RemoveClient 移除 WebSocket 客户端
func (s *Service) RemoveClient(client *ClientConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
}

// SendControl 发送控制消息
func (s *Service) SendControl(msg ControlMessage) error {
	s.mu.RLock()
	scrcpyClient := s.scrcpyClient
	running := s.running
	s.mu.RUnlock()

	if !running || scrcpyClient == nil {
		return fmt.Errorf("no active session")
	}

	switch msg.Type {
	case ControlTypeTouch:
		return s.handleTouch(scrcpyClient, msg.Data)
	case ControlTypeKey:
		return s.handleKey(scrcpyClient, msg.Data)
	case ControlTypeScroll:
		return s.handleScroll(scrcpyClient, msg.Data)
	case ControlTypeBack:
		// 发送 DOWN 和 UP 完成一次按键
		if err := scrcpyClient.BackOrScreenOn(scrcpy.KeyActionDown); err != nil {
			return err
		}
		return scrcpyClient.BackOrScreenOn(scrcpy.KeyActionUp)
	case ControlTypeHome:
		// KEYCODE_HOME = 3
		if err := scrcpyClient.InjectKeycode(scrcpy.KeyActionDown, 3, 0, 0); err != nil {
			return err
		}
		return scrcpyClient.InjectKeycode(scrcpy.KeyActionUp, 3, 0, 0)
	case ControlTypeRecent:
		// KEYCODE_APP_SWITCH = 187
		if err := scrcpyClient.InjectKeycode(scrcpy.KeyActionDown, 187, 0, 0); err != nil {
			return err
		}
		return scrcpyClient.InjectKeycode(scrcpy.KeyActionUp, 187, 0, 0)
	case ControlTypePower:
		// KEYCODE_POWER = 26
		if err := scrcpyClient.InjectKeycode(scrcpy.KeyActionDown, 26, 0, 0); err != nil {
			return err
		}
		return scrcpyClient.InjectKeycode(scrcpy.KeyActionUp, 26, 0, 0)
	case ControlTypeRotate:
		return scrcpyClient.RotateDevice()
	default:
		return fmt.Errorf("unknown control type: %s", msg.Type)
	}
}

// handleTouch 处理触摸事件
func (s *Service) handleTouch(client *scrcpy.Client, data interface{}) error {
	m, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid touch data")
	}

	action := scrcpy.ActionMove
	if a, ok := m["action"].(string); ok {
		switch a {
		case "down":
			action = scrcpy.ActionDown
		case "up":
			action = scrcpy.ActionUp
		case "move":
			action = scrcpy.ActionMove
		}
	}

	x := float32(0)
	y := float32(0)
	if v, ok := m["x"].(float64); ok {
		x = float32(v)
	}
	if v, ok := m["y"].(float64); ok {
		y = float32(v)
	}

	pressure := float32(1.0)
	if action == scrcpy.ActionUp {
		pressure = 0
	}

	return client.InjectTouch(action, x, y, pressure)
}

// handleKey 处理按键事件
func (s *Service) handleKey(client *scrcpy.Client, data interface{}) error {
	m, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid key data")
	}

	keycode := 0
	if v, ok := m["keycode"].(float64); ok {
		keycode = int(v)
	}

	action := scrcpy.KeyActionDown
	if a, ok := m["action"].(string); ok && a == "up" {
		action = scrcpy.KeyActionUp
	}

	return client.InjectKeycode(action, keycode, 0, 0)
}

// handleScroll 处理滚动事件
func (s *Service) handleScroll(client *scrcpy.Client, data interface{}) error {
	m, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid scroll data")
	}

	x := float32(0.5)
	y := float32(0.5)
	if v, ok := m["x"].(float64); ok {
		x = float32(v)
	}
	if v, ok := m["y"].(float64); ok {
		y = float32(v)
	}

	vScroll := 0
	if v, ok := m["deltaY"].(float64); ok {
		if v > 0 {
			vScroll = -1
		} else if v < 0 {
			vScroll = 1
		}
	}

	return client.InjectScroll(x, y, 0, vScroll)
}

// ==================== 单设备操作 ====================

// GetDevice 获取单个设备信息
func (s *Service) GetDevice(serial string) (*Device, error) {
	devices, err := s.GetDevices()
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		if d.ID == serial || d.Serial == serial {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("device not found: %s", serial)
}

// GetApps 获取设备上已安装的应用列表
func (s *Service) GetApps(serial string) ([]AndroidApp, error) {
	if err := s.initADBClient(); err != nil {
		return nil, fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// 获取第三方应用列表
	output, err := dc.RunShellCommand("pm", "list", "packages", "-3")
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	var apps []AndroidApp
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		pkg := strings.TrimPrefix(line, "package:")
		if pkg == "" || pkg == line {
			continue
		}

		app := AndroidApp{
			PackageName: pkg,
			AppName:     pkg, // 默认用包名
			Installed:   true,
		}

		// 尝试获取应用名称和版本
		if dump, err := dc.RunShellCommand("dumpsys", "package", pkg); err == nil {
			if ver := extractLine(dump, "versionName="); ver != "" {
				app.Version = ver
			}
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// InstallAPKToDevice 安装 APK 到指定设备
func (s *Service) InstallAPKToDevice(serial, filePath string) (*AndroidApp, error) {
	if err := s.initADBClient(); err != nil {
		return nil, fmt.Errorf("ADB not available: %w", err)
	}

	s.logger.Info("Installing APK", zap.String("path", filePath), zap.String("device", serial))
	output, err := exec.CommandContext(
		context.Background(),
		"adb", "-s", serial, "install", "-r", filePath,
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb install failed: %s - %w", string(output), err)
	}

	if !strings.Contains(string(output), "Success") {
		return nil, fmt.Errorf("install failed: %s", string(output))
	}

	apkInfo, _ := s.ParseAPK(filePath)
	app := &AndroidApp{
		AppName:   filepath.Base(filePath),
		Installed: true,
	}
	if apkInfo != nil {
		app.PackageName = apkInfo.PackageName
		app.AppName = apkInfo.AppName
		app.Version = apkInfo.VersionName
	}

	s.logger.Info("APK installed successfully", zap.String("package", app.PackageName))
	return app, nil
}

// UninstallApp 卸载应用
func (s *Service) UninstallApp(serial, packageName string) error {
	if err := s.initADBClient(); err != nil {
		return fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	output, err := dc.RunShellCommand("pm", "uninstall", packageName)
	if err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}
	if !strings.Contains(output, "Success") {
		return fmt.Errorf("uninstall failed: %s", output)
	}

	s.logger.Info("App uninstalled", zap.String("package", packageName))
	return nil
}

// LaunchApp 启动应用
func (s *Service) LaunchApp(serial, packageName string) error {
	if err := s.initADBClient(); err != nil {
		return fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// 使用 monkey 启动应用的主 Activity
	_, err = dc.RunShellCommand("monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
	if err != nil {
		return fmt.Errorf("launch app failed: %w", err)
	}

	s.logger.Info("App launched", zap.String("package", packageName))
	return nil
}

// StopApp 停止应用
func (s *Service) StopApp(serial, packageName string) error {
	if err := s.initADBClient(); err != nil {
		return fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	_, err = dc.RunShellCommand("am", "force-stop", packageName)
	if err != nil {
		return fmt.Errorf("stop app failed: %w", err)
	}

	return nil
}

// RebootDevice 重启设备
func (s *Service) RebootDevice(serial string) error {
	if err := s.initADBClient(); err != nil {
		return fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	_, err = dc.RunShellCommand("reboot")
	if err != nil {
		return fmt.Errorf("reboot failed: %w", err)
	}

	s.logger.Info("Device rebooting", zap.String("serial", serial))
	return nil
}

// Shell 执行 Shell 命令
func (s *Service) Shell(serial, command string) (string, error) {
	if err := s.initADBClient(); err != nil {
		return "", fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return "", fmt.Errorf("device not found: %w", err)
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	args := make([]string, 0)
	if len(parts) > 1 {
		args = parts[1:]
	}

	output, err := dc.RunShellCommand(parts[0], args...)
	if err != nil {
		return "", fmt.Errorf("shell command failed: %w", err)
	}

	return output, nil
}

// Screenshot 截图（返回 base64 PNG）
func (s *Service) Screenshot(serial string) (string, error) {
	if err := s.initADBClient(); err != nil {
		return "", fmt.Errorf("ADB not available: %w", err)
	}

	// 使用 adb exec-out screencap -p 获取截图
	output, err := exec.CommandContext(
		context.Background(),
		"adb", "-s", serial, "exec-out", "screencap", "-p",
	).Output()
	if err != nil {
		return "", fmt.Errorf("screenshot failed: %w", err)
	}

	encoded := "data:image/png;base64," + encodeBase64(output)
	return encoded, nil
}

// Input 向设备发送输入
func (s *Service) Input(serial, inputType, args string) error {
	if err := s.initADBClient(); err != nil {
		return fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	_, err = dc.RunShellCommand("input", inputType, args)
	if err != nil {
		return fmt.Errorf("input failed: %w", err)
	}

	return nil
}

// ListFiles 列出设备上的文件
func (s *Service) ListFiles(serial, path string) ([]FileInfo, error) {
	if err := s.initADBClient(); err != nil {
		return nil, fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	output, err := dc.RunShellCommand("ls", "-la", path)
	if err != nil {
		return nil, fmt.Errorf("list files failed: %w", err)
	}

	var files []FileInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		name := strings.Join(fields[7:], " ")
		isDir := strings.HasPrefix(fields[0], "d")

		size := int64(0)
		fmt.Sscanf(fields[4], "%d", &size)

		files = append(files, FileInfo{
			Name:  name,
			Path:  filepath.Join(path, name),
			Size:  size,
			IsDir: isDir,
		})
	}

	return files, nil
}

// PushFile 推送文件到设备
func (s *Service) PushFile(serial, localPath, remotePath string) error {
	if err := s.initADBClient(); err != nil {
		return fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	return dc.Push(localPath, remotePath)
}

// PullFile 从设备拉取文件
func (s *Service) PullFile(serial, remotePath, localPath string) error {
	if err := s.initADBClient(); err != nil {
		return fmt.Errorf("ADB not available: %w", err)
	}

	dc, err := s.adbClient.GetDevice(serial)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	data, err := dc.Pull(remotePath)
	if err != nil {
		return fmt.Errorf("pull file failed: %w", err)
	}

	return os.WriteFile(localPath, data, 0644)
}

// extractLine 从多行文本中提取包含关键字的行的值
func extractLine(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
		if idx := strings.Index(line, prefix); idx >= 0 {
			return strings.TrimSpace(line[idx+len(prefix):])
		}
	}
	return ""
}

// Close 关闭服务
func (s *Service) Close() {
	s.StopSession("")

	s.mu.Lock()
	defer s.mu.Unlock()

	for client := range s.clients {
		client.Close()
	}
	s.clients = make(map[*ClientConn]bool)
}

// ========== APK 管理 ==========

// ParseAPK 解析 APK 文件信息
func (s *Service) ParseAPK(filePath string) (*APKInfo, error) {
	if err := s.initADBClient(); err != nil {
		return nil, fmt.Errorf("ADB not available: %w", err)
	}

	var fileSize int64
	if info, err := os.Stat(filePath); err == nil {
		fileSize = info.Size()
	}

	info := &APKInfo{
		PackageName: "",
		AppName:     filepath.Base(filePath),
		Size:        fileSize,
	}

	if strings.HasSuffix(strings.ToLower(info.AppName), ".apk") {
		info.AppName = info.AppName[:len(info.AppName)-4]
	}

	// 尝试使用 aapt2/aapt 解析
	if output, err := exec.Command("aapt2", "dump", "badging", filePath).Output(); err == nil {
		parseAaptOutput(string(output), info)
	} else if output, err := exec.Command("aapt", "dump", "badging", filePath).Output(); err == nil {
		parseAaptOutput(string(output), info)
	}

	return info, nil
}

// parseAaptOutput 解析 aapt dump 输出
func parseAaptOutput(output string, info *APKInfo) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package:") {
			if m := extractAttr(line, "name"); m != "" {
				info.PackageName = m
			}
			if m := extractAttr(line, "versionName"); m != "" {
				info.VersionName = m
			}
			if m := extractAttr(line, "versionCode"); m != "" {
				fmt.Sscanf(m, "%d", &info.VersionCode)
			}
		} else if strings.HasPrefix(line, "application-label:") {
			if idx := strings.Index(line, "'"); idx >= 0 {
				end := strings.LastIndex(line, "'")
				if end > idx {
					info.AppName = line[idx+1 : end]
				}
			}
		}
	}
}

// extractAttr 从 aapt 输出行中提取属性值
func extractAttr(line, attr string) string {
	key := attr + "='"
	idx := strings.Index(line, key)
	if idx < 0 {
		return ""
	}
	start := idx + len(key)
	end := strings.Index(line[start:], "'")
	if end < 0 {
		return ""
	}
	return line[start : start+end]
}

// InstallAPK 安装 APK 到设备
func (s *Service) InstallAPK(filePath string) (*AndroidApp, error) {
	if err := s.initADBClient(); err != nil {
		return nil, fmt.Errorf("ADB not available: %w", err)
	}

	devices, err := s.adbClient.ListDevices()
	if err != nil || len(devices) == 0 {
		return nil, fmt.Errorf("no connected Android device")
	}

	serial := devices[0].Serial

	s.logger.Info("Installing APK", zap.String("path", filePath), zap.String("device", serial))
	output, err := exec.CommandContext(
		context.Background(),
		"adb", "-s", serial, "install", "-r", filePath,
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb install failed: %s - %w", string(output), err)
	}

	if !strings.Contains(string(output), "Success") {
		return nil, fmt.Errorf("install failed: %s", string(output))
	}

	apkInfo, _ := s.ParseAPK(filePath)

	app := &AndroidApp{
		PackageName: "",
		AppName:     filepath.Base(filePath),
		Installed:   true,
	}

	if apkInfo != nil {
		app.PackageName = apkInfo.PackageName
		app.AppName = apkInfo.AppName
		app.Version = apkInfo.VersionName
	}

	s.logger.Info("APK installed successfully",
		zap.String("package", app.PackageName),
		zap.String("name", app.AppName))

	return app, nil
}

// 辅助函数
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// encodeBase64 将字节数组编码为 base64 字符串
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// ========== 环境检测辅助函数 ==========

// CheckDKMSInstalled 检查 DKMS 是否已安装
func CheckDKMSInstalled() bool {
	_, err := exec.LookPath("dkms")
	return err == nil
}

// CheckLinuxHeadersInstalled 检查当前内核的 headers 是否已安装
func CheckLinuxHeadersInstalled() bool {
	return CheckKernelHeadersExist()
}

// GetKernelVersion 获取当前内核版本
func GetKernelVersion() string {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// CheckKernelHeadersExist 检查内核头文件是否存在
func CheckKernelHeadersExist() bool {
	kernelVersion := GetKernelVersion()
	if kernelVersion == "" {
		return false
	}

	paths := []string{
		"/lib/modules/" + kernelVersion + "/build",
		"/lib/modules/" + kernelVersion + "/source",
		"/usr/src/linux-headers-" + kernelVersion,
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// CheckBinderModuleLoaded 检查 binder 模块是否已加载
func CheckBinderModuleLoaded() bool {
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "binder_linux")
}

// CheckADBInstalled 检查 ADB 是否已安装
func CheckADBInstalled() bool {
	_, err := exec.LookPath("adb")
	return err == nil
}

// EnsureADBServer 确保 ADB server 正在运行，如果未运行则自动启动
func EnsureADBServer() error {
	if !CheckADBInstalled() {
		return fmt.Errorf("adb is not installed, please install android-tools-adb")
	}

	// 尝试启动 ADB server（如果已运行则不会重复启动）
	cmd := exec.Command("adb", "start-server")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start adb server: %w", err)
	}
	return nil
}

// CheckDockerInstalled 检查 Docker 是否已安装
func CheckDockerInstalled() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// CheckDockerImageExists 检查 Docker 镜像是否存在
func CheckDockerImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	return cmd.Run() == nil
}

// CheckContainerRunning 检查容器是否正在运行
func CheckContainerRunning(containerName string) bool {
	cmd := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// CheckContainerExists 检查容器是否存在
func CheckContainerExists(containerName string) bool {
	cmd := exec.Command("docker", "ps", "-aq", "-f", "name="+containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// StartContainer 启动 Android 容器
func (s *Service) StartContainer() error {
	containerName := "ruizios-android"

	// 已经在运行
	if CheckContainerRunning(containerName) {
		return nil
	}

	// 容器存在但未运行，启动它
	if CheckContainerExists(containerName) {
		cmd := exec.Command("docker", "start", containerName)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("启动容器失败: %w", err)
		}
		s.logger.Info("容器已启动", zap.String("container", containerName))
		return nil
	}

	return fmt.Errorf("容器不存在，请先安装")
}

// StopContainer 停止 Android 容器
func (s *Service) StopContainer() error {
	containerName := "ruizios-android"

	if !CheckContainerRunning(containerName) {
		return nil
	}

	// 先停止投屏
	s.StopSession("")

	// 停止容器
	cmd := exec.Command("docker", "stop", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("停止容器失败: %w", err)
	}

	s.logger.Info("容器已停止", zap.String("container", containerName))
	return nil
}
