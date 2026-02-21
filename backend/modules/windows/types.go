// Package windows Windows 类型定义
package windows

import "time"

// AppStatus 应用状态
type AppStatus string

const (
	AppStatusStopped    AppStatus = "stopped"
	AppStatusRunning    AppStatus = "running"
	AppStatusInstalling AppStatus = "installing"
	AppStatusError      AppStatus = "error"
)

// WinePrefix Wine 前缀
type WinePrefix struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Arch         string    `json:"arch"` // win32, win64
	WindowsVer   string    `json:"windows_version,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Size         int64     `json:"size,omitempty"`
}

// App Windows 应用
type App struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PrefixID    string    `json:"prefix_id"`
	ExePath     string    `json:"exe_path"`
	WorkDir     string    `json:"work_dir,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	Args        []string  `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Status      AppStatus `json:"status"`
	InstalledAt time.Time `json:"installed_at"`
}

// Session 运行会话
type Session struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	AppName   string    `json:"app_name"`
	PrefixID  string    `json:"prefix_id"`
	PID       int       `json:"pid"`
	Display   int       `json:"display"`
	Port      int       `json:"port"`
	Status    AppStatus `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// CreatePrefixRequest 创建前缀请求
type CreatePrefixRequest struct {
	Name       string `json:"name" binding:"required"`
	Arch       string `json:"arch,omitempty"` // win32, win64
	WindowsVer string `json:"windows_version,omitempty"`
}

// InstallAppRequest 安装应用请求
type InstallAppRequest struct {
	PrefixID    string `json:"prefix_id" binding:"required"`
	InstallerPath string `json:"installer_path" binding:"required"`
	Name        string `json:"name,omitempty"`
	Silent      bool   `json:"silent,omitempty"`
}

// AddAppRequest 添加应用请求
type AddAppRequest struct {
	PrefixID string            `json:"prefix_id" binding:"required"`
	Name     string            `json:"name" binding:"required"`
	ExePath  string            `json:"exe_path" binding:"required"`
	WorkDir  string            `json:"work_dir,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// UpdateAppRequest 更新应用请求
type UpdateAppRequest struct {
	Name    string            `json:"name,omitempty"`
	ExePath string            `json:"exe_path,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// LaunchRequest 启动请求
type LaunchRequest struct {
	AppID string            `json:"app_id" binding:"required"`
	Args  []string          `json:"args,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
}

// RunExeRequest 运行 EXE 请求
type RunExeRequest struct {
	PrefixID string            `json:"prefix_id" binding:"required"`
	ExePath  string            `json:"exe_path" binding:"required"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// WinetricksRequest Winetricks 请求
type WinetricksRequest struct {
	PrefixID  string   `json:"prefix_id" binding:"required"`
	Verbs     []string `json:"verbs" binding:"required"`
}

// WineConfig Wine 配置
type WineConfig struct {
	WindowsVersion string `json:"windows_version"`
	DPI            int    `json:"dpi"`
	VirtualDesktop string `json:"virtual_desktop,omitempty"` // 1024x768
	DXVK           bool   `json:"dxvk"`
	VKD3D          bool   `json:"vkd3d"`
	Gallium9       bool   `json:"gallium9"`
}

// StoreApp 应用商店应用
type StoreApp struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Category    string   `json:"category,omitempty"`
	Version     string   `json:"version,omitempty"`
	Source      string   `json:"source,omitempty"`
	Script      string   `json:"script,omitempty"`
	Verbs       []string `json:"verbs,omitempty"`
}

// WineInfo Wine 信息
type WineInfo struct {
	Version     string `json:"version"`
	Path        string `json:"path"`
	Arch        string `json:"arch"`
	WinetricksVer string `json:"winetricks_version,omitempty"`
	DXVKVersion string `json:"dxvk_version,omitempty"`
}
