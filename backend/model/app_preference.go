// Package model 应用偏好数据模型
package model

import (
	"time"

	"gorm.io/gorm"
)

// UserPreference 用户偏好配置
type UserPreference struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    string         `json:"user_id" gorm:"size:36;not null;index:idx_user_key,unique"`
	Key       string         `json:"key" gorm:"size:64;not null;index:idx_user_key,unique"`
	Value     string         `json:"value" gorm:"type:text"` // JSON 格式存储
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (UserPreference) TableName() string {
	return "user_preferences"
}

// 预定义的偏好 Key 常量
const (
	PrefKeyStartMenuPosition = "start_menu_position" // "left" | "center"
	PrefKeyPinnedApps        = "pinned_apps"         // JSON array of app IDs
	PrefKeyTaskbarApps       = "taskbar_apps"        // JSON array of {id, order}
	PrefKeyRecentApps        = "recent_apps"         // JSON array of app IDs (最近使用)
)

// DesktopIcon 桌面图标配置
type DesktopIcon struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    string         `json:"user_id" gorm:"size:36;not null;index:idx_desktop_user_app,unique"`
	AppID     string         `json:"app_id" gorm:"size:64;not null;index:idx_desktop_user_app,unique"`
	PositionX int            `json:"position_x" gorm:"not null;default:0"`
	PositionY int            `json:"position_y" gorm:"not null;default:0"`
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (DesktopIcon) TableName() string {
	return "desktop_icons"
}

// SystemApp 系统/模块应用注册表
type SystemApp struct {
	ID               uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	AppID            string         `json:"app_id" gorm:"size:64;not null;uniqueIndex"`
	Name             string         `json:"name" gorm:"size:128;not null"`
	Icon             string         `json:"icon" gorm:"size:512;not null"`
	Source           string         `json:"source" gorm:"size:32;not null;index"` // system, module, docker_store, linux_store, windows_store
	ModuleDependency string         `json:"module_dependency,omitempty" gorm:"size:64"`
	LaunchType       string         `json:"launch_type" gorm:"size:32;not null"` // internal, docker, linux, windows
	LaunchTarget     string         `json:"launch_target" gorm:"size:256;not null"`
	State            string         `json:"state" gorm:"size:32;not null;default:'active';index"` // active, module_disabled, installing, uninstalling
	Category         string         `json:"category,omitempty" gorm:"size:32"`                    // 开始菜单分类
	Keywords         string         `json:"keywords,omitempty" gorm:"size:256"`                   // 搜索关键词, 逗号分隔
	CreatedAt        time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (SystemApp) TableName() string {
	return "system_apps"
}

// SystemAppSource 系统应用来源类型
type SystemAppSource string

const (
	SystemAppSourceSystem       SystemAppSource = "system"
	SystemAppSourceModule       SystemAppSource = "module"
	SystemAppSourceDockerStore  SystemAppSource = "docker_store"
	SystemAppSourceLinuxStore   SystemAppSource = "linux_store"
	SystemAppSourceWindowsStore SystemAppSource = "windows_store"
)

// SystemAppState 系统应用状态类型
type SystemAppState string

const (
	SystemAppStateActive         SystemAppState = "active"
	SystemAppStateModuleDisabled SystemAppState = "module_disabled"
	SystemAppStateInstalling     SystemAppState = "installing"
	SystemAppStateUninstalling   SystemAppState = "uninstalling"
)

// SystemAppLaunchType 系统应用启动类型
type SystemAppLaunchType string

const (
	SystemAppLaunchTypeInternal SystemAppLaunchType = "internal"
	SystemAppLaunchTypeDocker   SystemAppLaunchType = "docker"
	SystemAppLaunchTypeLinux    SystemAppLaunchType = "linux"
	SystemAppLaunchTypeWindows  SystemAppLaunchType = "windows"
)

// ============ API 请求/响应结构 ============

// AppResponse 单个应用响应
type AppResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Icon             string `json:"icon"`
	Source           string `json:"source"`
	ModuleDependency string `json:"moduleDependency,omitempty"`
	LaunchConfig     struct {
		Type   string `json:"type"`
		Target string `json:"target"`
	} `json:"launchConfig"`
	State    string `json:"state"`
	Category string `json:"category,omitempty"`
	Keywords string `json:"keywords,omitempty"`
}

// AppsListResponse 应用列表响应
type AppsListResponse struct {
	Apps []AppResponse `json:"apps"`
}

// UserPreferencesResponse 用户偏好响应
type UserPreferencesResponse struct {
	StartMenuPosition string                    `json:"startMenuPosition"`
	PinnedApps        []string                  `json:"pinnedApps"`
	TaskbarApps       []TaskbarAppItem          `json:"taskbarApps"`
	DesktopIcons      []DesktopIconItem         `json:"desktopIcons"`
	RecentApps        []string                  `json:"recentApps"`
}

// TaskbarAppItem 任务栏应用项
type TaskbarAppItem struct {
	ID    string `json:"id"`
	Order int    `json:"order"`
}

// DesktopIconItem 桌面图标项
type DesktopIconItem struct {
	AppID string `json:"appId"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
}

// UpdatePreferencesRequest 更新偏好请求
type UpdatePreferencesRequest struct {
	StartMenuPosition *string          `json:"startMenuPosition,omitempty"`
	PinnedApps        []string         `json:"pinnedApps,omitempty"`
	TaskbarApps       []TaskbarAppItem `json:"taskbarApps,omitempty"`
	RecentApps        []string         `json:"recentApps,omitempty"`
}

// UpdateDesktopIconsRequest 更新桌面图标请求
type UpdateDesktopIconsRequest struct {
	Icons []DesktopIconItem `json:"icons" binding:"required"`
}

// AddDesktopIconRequest 添加桌面图标请求
type AddDesktopIconRequest struct {
	AppID string `json:"appId" binding:"required"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
}

// PinAppRequest 固定应用请求
type PinAppRequest struct {
	AppID string `json:"appId" binding:"required"`
}

// UpdateTaskbarRequest 更新任务栏请求
type UpdateTaskbarRequest struct {
	Apps []TaskbarAppItem `json:"apps" binding:"required"`
}
