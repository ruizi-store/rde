// Package setup 系统初始化向导模块
package setup

import "time"

// ----- 系统检查相关 -----

// SystemCheckResult 系统检查结果
type SystemCheckResult struct {
	Dependencies []DependencyCheck `json:"dependencies"`
	Ports        []PortCheck       `json:"ports"`
	DiskSpace    DiskSpaceCheck    `json:"disk_space"`
	AllPassed    bool              `json:"all_passed"`
}

// DependencyCheck 依赖检查项
type DependencyCheck struct {
	Name      string `json:"name"`
	Required  bool   `json:"required"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
}

// PortCheck 端口检查项
type PortCheck struct {
	Port         int    `json:"port"`
	InUse        bool   `json:"in_use"`
	InUseProcess string `json:"in_use_process"`
}

// DiskSpaceCheck 磁盘空间检查
type DiskSpaceCheck struct {
	Path        string `json:"path"`
	TotalBytes  int64  `json:"total_bytes"`
	AvailBytes  int64  `json:"avail_bytes"`
	MinRequired int64  `json:"min_required"`
	Sufficient  bool   `json:"sufficient"`
}

// ----- 语言时区相关 -----

// LocaleSettings 语言和时区设置
type LocaleSettings struct {
	Language   string `json:"language" binding:"required"`
	Timezone   string `json:"timezone" binding:"required"`
	TimeFormat string `json:"time_format" binding:"required"`
	DateFormat string `json:"date_format" binding:"required"`
}

// ----- 用户创建相关 -----

// SetupUserRequest 创建管理员请求
type SetupUserRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=32,alphanum"`
	Password  string `json:"password" binding:"required,min=8"`
	Avatar    string `json:"avatar,omitempty"`
	Enable2FA bool   `json:"enable_2fa"`
}

// TwoFactorSetup 2FA 配置响应
type TwoFactorSetup struct {
	Secret      string   `json:"secret"`
	QRCodeURL   string   `json:"qr_code_url"`
	BackupCodes []string `json:"backup_codes"`
}

// Verify2FARequest 验证 2FA 请求
type Verify2FARequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// ----- 存储配置相关 -----

// StorageConfig 存储配置（原有的简单配置，保持兼容）
type StorageConfig struct {
	DataPath       string       `json:"data_path" binding:"required"`
	ExternalDrives []DriveMount `json:"external_drives"`
}

// AvailableDisk 可用于创建存储池的硬盘
type AvailableDisk struct {
	Path      string `json:"path"`      // 设备路径
	Name      string `json:"name"`      // 设备名称
	Model     string `json:"model"`     // 型号
	Serial    string `json:"serial"`    // 序列号
	Size      uint64 `json:"size"`      // 大小
	Type      string `json:"type"`      // 类型: hdd, ssd, nvme
	Transport string `json:"transport"` // 传输协议: sata, nvme, usb
	IsSystem  bool   `json:"is_system"` // 是否为系统盘
	InUse     bool   `json:"in_use"`    // 是否已被使用
}

// DriveMount 硬盘挂载配置
type DriveMount struct {
	DevicePath string `json:"device_path" binding:"required"`
	MountPoint string `json:"mount_point" binding:"required"`
	Filesystem string `json:"filesystem"`
	Label      string `json:"label"`
	AutoMount  bool   `json:"auto_mount"`
}

// DetectedDrive 检测到的硬盘
type DetectedDrive struct {
	DevicePath string      `json:"device_path"`
	Size       int64       `json:"size"`
	Model      string      `json:"model"`
	Serial     string      `json:"serial"`
	Partitions []Partition `json:"partitions"`
}

// Partition 分区信息
type Partition struct {
	DevicePath string `json:"device_path"`
	Size       int64  `json:"size"`
	Filesystem string `json:"filesystem"`
	MountPoint string `json:"mount_point"`
	Label      string `json:"label"`
}

// ----- 网络配置相关 -----

// NetworkConfig 网络配置
type NetworkConfig struct {
	Mode      string   `json:"mode,omitempty" binding:"omitempty,oneof=dhcp static"` // dhcp | static，默认 dhcp
	IPAddress string   `json:"ip_address,omitempty"`
	Netmask   string   `json:"netmask,omitempty"`
	Gateway   string   `json:"gateway,omitempty"`
	DNS       []string `json:"dns,omitempty"`
	HTTPPort  int      `json:"http_port,omitempty"`
	HTTPSPort int      `json:"https_port,omitempty"`
}

// ----- 功能选择相关 (Step 6) -----

// FeatureSelection 功能选择请求
type FeatureSelection struct {
	EnabledModules []string `json:"enabled_modules" binding:"required"` // 启用的模块 ID 列表
}

// FeatureOption 可选功能项
type FeatureOption struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Icon         string   `json:"icon,omitempty"`
	Category     string   `json:"category"`     // 分类: container, virtualization, tools
	Dependencies []string `json:"dependencies"` // 依赖的模块
	Recommended  bool     `json:"recommended"`  // 是否推荐启用
	RequiresHW   []string `json:"requires_hw"`  // 需要的硬件: kvm, docker, gpu
}

// FeatureOptionsResponse 可选功能列表响应
type FeatureOptionsResponse struct {
	Categories []FeatureCategory `json:"categories"`
	Features   []FeatureOption   `json:"features"`
	HWSupport  HardwareSupport   `json:"hw_support"` // 硬件支持情况
}

// FeatureCategory 功能分类
type FeatureCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// HardwareSupport 硬件支持检测结果
type HardwareSupport struct {
	KVMAvailable    bool   `json:"kvm_available"`
	DockerAvailable bool   `json:"docker_available"`
	GPUAvailable    bool   `json:"gpu_available"`
	GPUType         string `json:"gpu_type,omitempty"` // nvidia, amd, intel
}

// ----- 初始化状态相关 -----

// SetupStatus 初始化状态
type SetupStatus struct {
	Completed      bool  `json:"completed"`
	CurrentStep    int   `json:"current_step"`
	CompletedSteps []int `json:"completed_steps"`
	CanSkipSetup   bool  `json:"can_skip_setup"`
}

// SetupSettings 初始化设置（存储在数据库中）
type SetupSettings struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	SetupCompleted bool       `json:"setup_completed" gorm:"default:false"`
	CurrentStep    int        `json:"current_step" gorm:"default:1"`
	CompletedSteps string     `json:"completed_steps" gorm:"type:text"` // JSON 数组
	Language       string     `json:"language" gorm:"size:10;default:'zh-CN'"`
	Timezone       string     `json:"timezone" gorm:"size:64;default:'Asia/Shanghai'"`
	TimeFormat     string     `json:"time_format" gorm:"size:10;default:'24h'"`
	DateFormat     string     `json:"date_format" gorm:"size:20;default:'YYYY-MM-DD'"`
	DataPath       string     `json:"data_path" gorm:"size:255"`
	NetworkMode    string     `json:"network_mode" gorm:"size:16;default:'dhcp'"`
	HTTPPort       int        `json:"http_port" gorm:"default:80"`
	HTTPSPort      int        `json:"https_port" gorm:"default:443"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SetupSettings) TableName() string {
	return "setup_settings"
}

// InstallDepsRequest 安装依赖请求
type InstallDepsRequest struct {
	Packages []string `json:"packages" binding:"required,min=1"`
}

// CompleteResponse 完成初始化响应
type CompleteResponse struct {
	Success         bool   `json:"success"`
	RedirectURL     string `json:"redirect_url"`
	AutoLoginToken  string `json:"auto_login_token,omitempty"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	TokenExpiresAt  int64  `json:"token_expires_at,omitempty"`
}

// ----- 安全配置相关 (Setup Step 2 扩展) -----

// SecurityConfig 安全配置请求
type SecurityConfig struct {
	UseRandomPort bool `json:"use_random_port"` // 是否使用随机端口
	CustomPort    int  `json:"custom_port"`     // 自定义端口（0 表示不使用）
}

// SecurityConfigResponse 安全配置响应
type SecurityConfigResponse struct {
	PortChanged bool   `json:"port_changed"`
	NewPort     int    `json:"new_port,omitempty"`
	Message     string `json:"message"`
}

// ----- 恢复出厂设置相关 -----

// FactoryResetRequest 恢复出厂设置请求
// 用户身份通过 JWT 获取，但需要再次输入密码确认
type FactoryResetRequest struct {
	Password       string `json:"password" binding:"required"`     // 当前用户密码（二次确认）
	ConfirmText    string `json:"confirm_text" binding:"required"` // 必须输入 "RESET"
	KeepDockerApps bool   `json:"keep_docker_apps"`                // 是否保留 Docker 应用
	KeepUserFiles  bool   `json:"keep_user_files"`                 // 是否保留用户文件
}

// FactoryResetResponse 恢复出厂设置响应
type FactoryResetResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	RedirectURL string `json:"redirect_url"`
}
