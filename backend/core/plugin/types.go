// Package plugin 提供外部插件的发现、加载和生命周期管理
// 插件是独立的可执行文件，通过 Unix Socket HTTP 与 RDE 主进程通信
package plugin

import "time"

// Manifest 描述插件的能力和元数据
// 每个插件目录下必须包含一个 manifest.json 文件
type Manifest struct {
	// ID 插件唯一标识符，如 "premium", "my-plugin"
	ID string `json:"id"`

	// Name 插件显示名称
	Name string `json:"name"`

	// Version 插件版本（语义化版本）
	Version string `json:"version"`

	// Description 插件描述
	Description string `json:"description,omitempty"`

	// Binary 可执行文件名，默认 "plugin"
	Binary string `json:"binary,omitempty"`

	// MinRDEVersion 要求的最低 RDE 版本
	MinRDEVersion string `json:"min_rde_version,omitempty"`

	// Routes 插件处理的路由前缀列表，如 ["/android/*", "/vm/*"]
	// 这些路由将被注册到 /api/v1 组下
	Routes []string `json:"routes"`

	// PublicRoutes 不需要认证的路由前缀
	PublicRoutes []string `json:"public_routes,omitempty"`

	// RequiresLicense 是否需要许可证
	RequiresLicense bool `json:"requires_license,omitempty"`

	// Apps 插件提供的前端应用列表
	// 每个应用将在桌面环境中显示为独立的可启动应用
	// 前端资源由插件进程自行服务（通过路由代理）
	Apps []PluginApp `json:"apps,omitempty"`
}

// PluginApp 描述插件提供的一个前端应用
type PluginApp struct {
	// ID 应用唯一标识（在桌面环境中的 appId）
	ID string `json:"id"`

	// Name 应用显示名称
	Name string `json:"name"`

	// Icon 应用图标（URL 路径或 Iconify 名称，如 "/icons/ai-assistant.svg" 或 "mdi:robot"）
	Icon string `json:"icon"`

	// FrontendRoute 前端资源的完整路由路径
	// 例如 "/app/ai/" 表示前端 SPA 入口在 /app/ai/
	FrontendRoute string `json:"frontend_route"`

	// Category 应用在开始菜单中的分类
	// 可选值: system, productivity, multimedia, network, tools, other
	Category string `json:"category,omitempty"`

	// DefaultWidth 默认窗口宽度
	DefaultWidth int `json:"default_width,omitempty"`

	// DefaultHeight 默认窗口高度
	DefaultHeight int `json:"default_height,omitempty"`

	// MinWidth 最小窗口宽度
	MinWidth int `json:"min_width,omitempty"`

	// MinHeight 最小窗口高度
	MinHeight int `json:"min_height,omitempty"`

	// Singleton 是否只允许打开一个实例
	Singleton bool `json:"singleton,omitempty"`

	// Permissions 应用需要的权限列表
	Permissions []string `json:"permissions,omitempty"`
}

// State 插件运行状态
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateError    State = "error"
)

// Info 插件运行时信息（用于 API 响应）
type Info struct {
	Manifest  Manifest   `json:"manifest"`
	State     State      `json:"state"`
	Socket    string     `json:"socket"`
	PID       int        `json:"pid,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
}

// PluginAppInfo 插件应用信息（用于前端发现）
type PluginAppInfo struct {
	PluginID string    `json:"plugin_id"`
	App      PluginApp `json:"app"`
}
