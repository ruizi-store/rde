// Package android Android 设备管理模块 - 类型定义
package android

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Device 设备信息
type Device struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Serial         string `json:"serial"`
	Model          string `json:"model"`
	Brand          string `json:"brand"`
	AndroidVersion string `json:"android_version,omitempty"`
	Status         string `json:"status"`
	Connected      bool   `json:"connected"`
}

// Session 投屏会话
type Session struct {
	ID           string    `json:"id"`
	DeviceSerial string    `json:"device_serial"`
	StartedAt    time.Time `json:"started_at"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Bitrate      int       `json:"bitrate"`
	MaxFps       int       `json:"max_fps"`
	VideoCodec   string    `json:"video_codec"`
	AudioEnabled bool      `json:"audio_enabled"`
	Status       string    `json:"status"`
}

// ScrcpyConfig 投屏配置
type ScrcpyConfig struct {
	MaxSize      int    `json:"maxSize"`
	Bitrate      int    `json:"bitrate"`
	MaxFps       int    `json:"maxFps"`
	VideoCodec   string `json:"videoCodec"`
	AudioCodec   string `json:"audioCodec"`
	AudioEnabled bool   `json:"audioEnabled"`
	TurnScreen   bool   `json:"turnScreen"`
	StayAwake    bool   `json:"stayAwake"`
	ShowTouches  bool   `json:"showTouches"`
	Orientation  string `json:"orientation"`
}

// DefaultScrcpyConfig 默认投屏配置
func DefaultScrcpyConfig() ScrcpyConfig {
	return ScrcpyConfig{
		MaxSize:      1920,
		Bitrate:      8000000,
		MaxFps:       60,
		VideoCodec:   "h264",
		AudioCodec:   "aac",
		AudioEnabled: false,
		TurnScreen:   false,
		StayAwake:    true,
		ShowTouches:  false,
		Orientation:  "",
	}
}

// TouchEvent 触摸事件
type TouchEvent struct {
	Action   string  `json:"action"` // down, up, move
	X        float64 `json:"x"`      // 归一化坐标 0~1
	Y        float64 `json:"y"`
	Pressure float64 `json:"pressure,omitempty"`
}

// KeyEvent 按键事件
type KeyEvent struct {
	Action  string `json:"action"` // down, up
	Keycode int    `json:"keycode"`
}

// ScrollEvent 滚动事件
type ScrollEvent struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	DeltaY float64 `json:"deltaY"`
}

// ControlMessage 控制消息
type ControlMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// WebSocket 消息类型
const (
	MsgTypeVideo      = "video"
	MsgTypeAudio      = "audio"
	MsgTypeControl    = "control"
	MsgTypeDeviceInfo = "device_info"
	MsgTypeConfig     = "config"
	MsgTypeError      = "error"
)

// 控制消息类型
const (
	ControlTypeTouch  = "touch"
	ControlTypeKey    = "key"
	ControlTypeScroll = "scroll"
	ControlTypeBack   = "back"
	ControlTypeHome   = "home"
	ControlTypeRecent = "recent"
	ControlTypePower  = "power"
	ControlTypeRotate = "rotate"
)

// APKInfo APK 文件信息
type APKInfo struct {
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name"`
	VersionName string `json:"version_name,omitempty"`
	VersionCode int    `json:"version_code,omitempty"`
	Size        int64  `json:"size"`
}

// AndroidApp 已安装的 Android 应用
type AndroidApp struct {
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name"`
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
}

// FileInfo 设备文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time,omitempty"`
}

// ClientConn 客户端连接
type ClientConn struct {
	Conn      *websocket.Conn
	SessionID string
	mu        sync.Mutex
	closed    bool
}

// Send 发送 JSON 消息
func (c *ClientConn) Send(msg interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	return c.Conn.WriteJSON(msg)
}

// SendBinary 发送二进制消息
func (c *ClientConn) SendBinary(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	return c.Conn.WriteMessage(websocket.BinaryMessage, data)
}

// Close 关闭连接
func (c *ClientConn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.Conn.Close()
	}
}

// APIResponse API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// InstallStep 安装步骤
type InstallStep string

const (
	StepCheckDependencies InstallStep = "check_dependencies"
	StepInstallDKMS       InstallStep = "install_dkms"
	StepInstallHeaders    InstallStep = "install_headers"
	StepInstallBinder     InstallStep = "install_binder"
	StepLoadBinder        InstallStep = "load_binder"
	StepInstallDocker     InstallStep = "install_docker"
	StepPullImage         InstallStep = "pull_image"
	StepStartContainer    InstallStep = "start_container"
	StepCompleted         InstallStep = "completed"
)

// StepStatus 步骤状态
type StepStatus string

const (
	StatusPending    StepStatus = "pending"
	StatusInProgress StepStatus = "in_progress"
	StatusCompleted  StepStatus = "completed"
	StatusFailed     StepStatus = "failed"
	StatusSkipped    StepStatus = "skipped"
)

// StepInfo 步骤信息
type StepInfo struct {
	Step        InstallStep `json:"step"`
	Status      StepStatus  `json:"status"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Progress    int         `json:"progress"`
	Error       string      `json:"error,omitempty"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	FinishedAt  *time.Time  `json:"finished_at,omitempty"`
}

// EnvironmentStatus 环境状态
type EnvironmentStatus struct {
	DKMSInstalled          bool          `json:"dkms_installed"`
	HeadersInstalled       bool          `json:"headers_installed"`
	BinderInstalled        bool          `json:"binder_installed"`
	BinderLoaded           bool          `json:"binder_loaded"`
	DockerInstalled        bool          `json:"docker_installed"`
	ImageExists            bool          `json:"image_exists"`
	ContainerRunning       bool          `json:"container_running"`
	ContainerExists        bool          `json:"container_exists"`
	KernelVersion          string        `json:"kernel_version"`
	RequiredSteps          []InstallStep `json:"required_steps"`
	IsReady                bool          `json:"is_ready"`
	OnlyNeedStartContainer bool          `json:"only_need_start_container"`
}

// InstallConfig 安装配置
type InstallConfig struct {
	DockerImage      string `json:"docker_image"`
	ContainerName    string `json:"container_name"`
	BinderModulePath string `json:"binder_module_path"`
	ADBPort          int    `json:"adb_port"`
	DataVolume       string `json:"data_volume"`
}
