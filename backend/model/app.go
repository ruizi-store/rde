package model

import "time"

// AppStoreSource 应用源配置
type AppStoreSource struct {
	Name string `json:"name" yaml:"name"` // 源名称
	URL  string `json:"url" yaml:"url"`   // 源地址 (GitHub raw URL)
}

// AppSource 应用来源追踪（转换工具生成）
type AppSource struct {
	Provider   string `json:"provider" yaml:"provider"`       // 1panel, casaos, portainer, rde
	OriginalID string `json:"original_id" yaml:"original_id"` // 原始应用 ID
	SyncDate   string `json:"sync_date" yaml:"sync_date"`     // 同步日期
}

// I18nContent 多语言内容
type I18nContent struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"` // 详细描述
	Tagline     string `json:"tagline,omitempty" yaml:"tagline,omitempty"`         // 简短标语
}

// AppRequirements 硬件要求
type AppRequirements struct {
	MinMemory     int      `json:"min_memory,omitempty" yaml:"min_memory,omitempty"`       // 最小内存 (MB)
	MinStorage    int      `json:"min_storage,omitempty" yaml:"min_storage,omitempty"`     // 最小存储 (MB)
	Architectures []string `json:"architectures,omitempty" yaml:"architectures,omitempty"` // amd64, arm64, arm/v7
	GPUSupport    bool     `json:"gpu_support,omitempty" yaml:"gpu_support,omitempty"`     // 是否支持 GPU
}

// App 应用商店中的应用定义
type App struct {
	// 核心标识
	ID      string `json:"id" yaml:"id"`           // 应用唯一标识
	Name    string `json:"name" yaml:"name"`       // 应用名称
	Version string `json:"version" yaml:"version"` // 应用版本
	Icon    string `json:"icon" yaml:"icon"`       // 图标URL

	// 来源追踪（转换工具生成）
	Source *AppSource `json:"source,omitempty" yaml:"source,omitempty"`

	// 分类与标签
	Category string   `json:"category" yaml:"category"` // 分类
	Tags     []string `json:"tags" yaml:"tags"`         // 标签

	// 多语言支持
	I18n map[string]*I18nContent `json:"i18n,omitempty" yaml:"i18n,omitempty"`

	// 简短描述
	Description string `json:"description" yaml:"description"`

	// 元数据
	Developer  string `json:"developer" yaml:"developer"`                   // 开发者
	Website    string `json:"website,omitempty" yaml:"website,omitempty"`   // 官方网站
	Repository string `json:"repository" yaml:"repository"`                 // 源码仓库
	Document   string `json:"document,omitempty" yaml:"document,omitempty"` // 文档链接
	License    string `json:"license,omitempty" yaml:"license,omitempty"`   // 开源协议

	// 硬件要求
	Requirements *AppRequirements `json:"requirements,omitempty" yaml:"requirements,omitempty"`

	// 截图
	Screenshots []string `json:"screenshots" yaml:"screenshots"`

	// Docker Compose 配置 (动态解析)
	Compose map[string]interface{} `json:"compose" yaml:"compose"`

	// 配置表单
	Form []AppFormField `json:"form" yaml:"form"`

	// 端口映射定义
	Ports []AppPort `json:"ports" yaml:"ports"`

	// 数据卷定义
	Volumes []AppVolume `json:"volumes" yaml:"volumes"`

	// 环境变量
	Environment map[string]string `json:"environment" yaml:"environment"`

	// 依赖的其他应用
	Depends []string `json:"depends" yaml:"depends"`

	// Web UI 地址模板
	WebUI string `json:"web_ui" yaml:"web_ui"`
}

// AppFormField 配置表单字段
type AppFormField struct {
	// 字段标识
	Key string `json:"key" yaml:"key"`

	// 显示标签（多语言）
	Label map[string]string `json:"label,omitempty" yaml:"label,omitempty"`

	// 字段描述
	Description string `json:"description" yaml:"description"`

	// 类型: text, number, password, select, boolean, path
	Type string `json:"type" yaml:"type"`

	// 默认值
	Default interface{} `json:"default" yaml:"default"`

	// 是否必填
	Required bool `json:"required" yaml:"required"`

	// select类型的选项
	Options []string `json:"options" yaml:"options"`

	// number类型的最小值
	Min int `json:"min" yaml:"min"`

	// number类型的最大值
	Max int `json:"max" yaml:"max"`

	// 占位符
	Placeholder string `json:"placeholder" yaml:"placeholder"`
}

// AppPort 端口定义
type AppPort struct {
	Container int    `json:"container" yaml:"container"` // 容器端口
	Host      int    `json:"host" yaml:"host"`           // 主机端口 (默认值)
	Protocol  string `json:"protocol" yaml:"protocol"`   // tcp/udp
	Label     string `json:"label" yaml:"label"`         // 端口用途说明
}

// AppVolume 数据卷定义
type AppVolume struct {
	Container string `json:"container" yaml:"container"` // 容器路径
	Host      string `json:"host" yaml:"host"`           // 主机路径模板
	Label     string `json:"label" yaml:"label"`         // 用途说明
	ReadOnly  bool   `json:"read_only" yaml:"read_only"` // 是否只读
}

// InstalledApp 已安装的应用
type InstalledApp struct {
	ID            uint              `json:"id" gorm:"primaryKey"`
	AppID         string            `json:"app_id" gorm:"index;not null"`    // 应用ID
	Name          string            `json:"name" gorm:"not null"`            // 应用名称
	Version       string            `json:"version"`                         // 安装版本
	Icon          string            `json:"icon"`                            // 图标
	Status        string            `json:"status" gorm:"default:'stopped'"` // 状态: installing, running, stopped, error
	ContainerIDs  string            `json:"container_ids"`                   // 容器ID列表 (JSON)
	NetworkID     string            `json:"network_id"`                      // Docker网络ID
	Config        string            `json:"config"`                          // 用户配置 (JSON)
	Ports         string            `json:"ports"`                           // 端口映射 (JSON)
	DataPath      string            `json:"data_path"`                       // 数据存储路径
	WebUI         string            `json:"web_ui"`                          // Web UI 地址
	ErrorMessage  string            `json:"error_message"`                   // 错误信息
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	ContainerList []string          `json:"container_list" gorm:"-"`           // 容器ID列表 (运行时)
	PortsMap      map[string]string `json:"ports_map" gorm:"-"`                // 端口映射 (运行时)
	ConfigMap     map[string]string `json:"config_map" gorm:"-"`               // 配置映射 (运行时)
	ResourceUsage *ResourceUsage    `json:"resource_usage,omitempty" gorm:"-"` // 资源使用情况 (运行时)
}

// ResourceUsage 应用资源使用情况
type ResourceUsage struct {
	CPU           string `json:"cpu"`            // CPU 使用率，如 "2.5%"
	Memory        string `json:"memory"`         // 内存使用量，如 "156MB"
	MemoryPercent string `json:"memory_percent"` // 内存使用率，如 "3.2%"
	NetworkRx     string `json:"network_rx"`     // 网络接收，如 "1.2GB"
	NetworkTx     string `json:"network_tx"`     // 网络发送，如 "500MB"
}

// TableName 指定表名
func (InstalledApp) TableName() string {
	return "installed_apps"
}

// AppInstallRequest 安装应用请求
type AppInstallRequest struct {
	AppID  string            `json:"app_id" validate:"required"`
	Config map[string]string `json:"config"` // 用户配置
	Ports  map[string]int    `json:"ports"`  // 自定义端口映射
}

// AppContainerStatus 容器状态
type AppContainerStatus struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	State       string `json:"state"`
	Health      string `json:"health"`
	CPUPercent  string `json:"cpu_percent"`
	MemUsage    string `json:"mem_usage"`
	NetIO       string `json:"net_io"`
	BlockIO     string `json:"block_io"`
}

// AppStatus 应用详细状态
type AppStatus struct {
	InstalledApp
	Containers []AppContainerStatus `json:"containers"` // 容器状态列表
}

// AppCategory 应用分类
type AppCategory struct {
	ID    string `json:"id" yaml:"id"`
	Name  string `json:"name" yaml:"name"`
	Icon  string `json:"icon" yaml:"icon"`
	Count int    `json:"count" yaml:"-"`
}

// AppIndex 应用源索引
type AppIndex struct {
	Version    string        `json:"version" yaml:"version"`
	UpdatedAt  string        `json:"updated_at" yaml:"updated_at"`
	Categories []AppCategory `json:"categories" yaml:"categories"`
	Apps       []App         `json:"apps" yaml:"apps"`
}

// PortAllocation 端口分配记录
type PortAllocation struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Port        int    `json:"port" gorm:"uniqueIndex;not null"`
	AppID       string `json:"app_id" gorm:"index;not null"`
	ServiceName string `json:"service_name"`
	Protocol    string `json:"protocol" gorm:"default:'tcp'"`
	Description string `json:"description"`
}

// TableName 指定表名
func (PortAllocation) TableName() string {
	return "port_allocations"
}
