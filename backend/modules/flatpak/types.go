// Package flatpak 类型定义
package flatpak

import "time"

// DesktopStatus 桌面实例状态
type DesktopStatus struct {
	Running        bool     `json:"running"`
	Display        int      `json:"display"`
	WebSocketPort  int      `json:"websocket_port"`
	VNCURL         string   `json:"vnc_url"`
	PID            int      `json:"pid"`
	Uptime         int64    `json:"uptime"`
	Resolution     string   `json:"resolution"`
	RunningApps    []string `json:"running_apps"`
	KasmVNCVersion string   `json:"kasmvnc_version"`
}

// DesktopConfig 桌面配置
type DesktopConfig struct {
	Display           int    `json:"display"`
	WebSocketPort     int    `json:"websocket_port"`
	DefaultResolution string `json:"default_resolution"`
	AudioEnabled      bool   `json:"audio_enabled"`
	ClipboardSync     bool   `json:"clipboard_sync"`
	AutoStart         bool   `json:"auto_start"`
}

// DefaultDesktopConfig 默认桌面配置
func DefaultDesktopConfig() DesktopConfig {
	return DesktopConfig{
		Display:           100,
		WebSocketPort:     6100,
		DefaultResolution: "1920x1080",
		AudioEnabled:      true,
		ClipboardSync:     true,
		AutoStart:         true,
	}
}

// FlatpakApp Flatpak 应用
type FlatpakApp struct {
	AppID       string `json:"app_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category,omitempty"`
	Installed   bool   `json:"installed"`
	Size        string `json:"size,omitempty"`
	Runtime     string `json:"runtime,omitempty"`
	Remote      string `json:"remote,omitempty"`
	Running     bool   `json:"running"`
}

// SetupStatus 环境检测状态
type SetupStatus struct {
	KasmVNCInstalled    bool   `json:"kasmvnc_installed"`
	KasmVNCVersion      string `json:"kasmvnc_version"`
	KasmVNCExpected     string `json:"kasmvnc_expected"`
	FlatpakInstalled    bool   `json:"flatpak_installed"`
	FlatpakRemoteOK     bool   `json:"flatpak_remote_ok"`
	OpenboxInstalled    bool   `json:"openbox_installed"`
	PulseAudioInstalled bool   `json:"pulseaudio_installed"`
	PulseAudioRunning   bool   `json:"pulseaudio_running"`
	VirtualSinkReady    bool   `json:"virtual_sink_ready"`
	Ready               bool   `json:"ready"`
}

// RunRequest 启动应用请求
type RunRequest struct {
	AppID string   `json:"app_id" binding:"required"`
	Args  []string `json:"args,omitempty"`
}

// InstallRequest 安装请求
type InstallRequest struct {
	AppID   string `json:"app_id" binding:"required"`
	AppName string `json:"app_name"`
}

// RecommendedApp 推荐应用
type RecommendedApp struct {
	AppID       string `json:"app_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon,omitempty"`
}

// RunningApp 运行中的应用进程
type RunningApp struct {
	AppID     string    `json:"app_id"`
	Name      string    `json:"name"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// ActiveInstall 正在进行的安装任务
type ActiveInstall struct {
	AppID     string    `json:"app_id"`
	AppName   string    `json:"app_name"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"` // installing, success, error
	Error     string    `json:"error,omitempty"`
}

// kasmVNCVersion 期望的 KasmVNC 版本
const kasmVNCVersion = "1.4.0"
