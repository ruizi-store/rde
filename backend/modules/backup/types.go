// Package backup 提供备份还原功能
package backup

import "time"

// BackupType 备份类型
type BackupType string

const (
	BackupTypeFull        BackupType = "full"        // 完整备份
	BackupTypeIncremental BackupType = "incremental" // 增量备份
	BackupTypeConfig      BackupType = "config"      // 配置备份
)

// BackupStatus 备份状态
type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"
	BackupStatusRunning   BackupStatus = "running"
	BackupStatusSuccess   BackupStatus = "success"
	BackupStatusFailed    BackupStatus = "failed"
	BackupStatusCancelled BackupStatus = "cancelled"
)

// TargetType 备份目标类型
type TargetType string

const (
	TargetTypeLocal  TargetType = "local"
	TargetTypeS3     TargetType = "s3"
	TargetTypeWebDAV TargetType = "webdav"
	TargetTypeSFTP   TargetType = "sftp"
)

// BackupTask 备份任务
type BackupTask struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Type         BackupType `json:"type"`
	Sources      []string   `json:"sources"`       // 备份源路径
	TargetType   TargetType `json:"target_type"`   // 目标类型
	TargetConfig string     `json:"target_config"` // 目标配置 JSON
	Schedule     string     `json:"schedule"`      // cron 表达式，为空则手动触发
	Retention    int        `json:"retention"`     // 保留份数
	Compression  bool       `json:"compression"`   // 是否压缩
	Encryption   bool       `json:"encryption"`    // 是否加密
	Enabled      bool       `json:"enabled"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// BackupRecord 备份记录
type BackupRecord struct {
	ID          string       `json:"id"`
	TaskID      string       `json:"task_id"`
	TaskName    string       `json:"task_name,omitempty"` // 关联显示
	Type        BackupType   `json:"type"`
	Size        int64        `json:"size"`        // 备份大小（字节）
	FileCount   int          `json:"file_count"`  // 文件数量
	FilePath    string       `json:"file_path"`   // 备份文件路径
	Checksum    string       `json:"checksum"`    // SHA256
	Status      BackupStatus `json:"status"`
	Progress    int          `json:"progress"`    // 进度 0-100
	Message     string       `json:"message"`     // 状态消息
	Error       string       `json:"error,omitempty"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// RestoreRequest 还原请求
type RestoreRequest struct {
	RecordID    string   `json:"record_id"`              // 备份记录ID
	TargetPath  string   `json:"target_path,omitempty"`  // 还原目标路径，空则还原到原位置
	SelectedItems []string `json:"selected_items,omitempty"` // 选择性还原的项
	Overwrite   bool     `json:"overwrite"`              // 是否覆盖已存在文件
}

// RestoreStatus 还原状态
type RestoreStatus struct {
	ID          string       `json:"id"`
	RecordID    string       `json:"record_id"`
	Status      BackupStatus `json:"status"`
	Progress    int          `json:"progress"`
	CurrentFile string       `json:"current_file,omitempty"`
	Message     string       `json:"message"`
	Error       string       `json:"error,omitempty"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// LocalTargetConfig 本地目标配置
type LocalTargetConfig struct {
	Path string `json:"path"`
}

// S3TargetConfig S3目标配置
type S3TargetConfig struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix,omitempty"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	UseSSL          bool   `json:"use_ssl"`
}

// WebDAVTargetConfig WebDAV目标配置
type WebDAVTargetConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Path     string `json:"path,omitempty"`
}

// SFTPTargetConfig SFTP目标配置
type SFTPTargetConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Path       string `json:"path"`
}

// TargetTestRequest 测试目标连接请求
type TargetTestRequest struct {
	Type   TargetType `json:"type"`
	Config string     `json:"config"` // JSON 配置
}

// TargetTestResponse 测试目标连接响应
type TargetTestResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	FreeSpace int64  `json:"free_space,omitempty"` // 可用空间（字节）
}

// BackupOverview 备份概览
type BackupOverview struct {
	TotalTasks     int   `json:"total_tasks"`
	EnabledTasks   int   `json:"enabled_tasks"`
	TotalRecords   int   `json:"total_records"`
	TotalSize      int64 `json:"total_size"`
	LastBackupAt   *time.Time `json:"last_backup_at,omitempty"`
	NextBackupAt   *time.Time `json:"next_backup_at,omitempty"`
	SuccessCount   int   `json:"success_count"`
	FailedCount    int   `json:"failed_count"`
}

// ConfigExportRequest 配置导出请求
type ConfigExportRequest struct {
	IncludeItems []string `json:"include_items"` // 要导出的配置项
	Encryption   bool     `json:"encryption"`
	Password     string   `json:"password,omitempty"`
}

// ConfigImportRequest 配置导入请求
type ConfigImportRequest struct {
	Password  string `json:"password,omitempty"`
	Overwrite bool   `json:"overwrite"`
}

// ExportableConfig 可导出的配置项
type ExportableConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"` // system, app, user
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Name         string     `json:"name" binding:"required"`
	Description  string     `json:"description"`
	Type         BackupType `json:"type" binding:"required"`
	Sources      []string   `json:"sources" binding:"required"`
	TargetType   TargetType `json:"target_type" binding:"required"`
	TargetConfig string     `json:"target_config" binding:"required"`
	Schedule     string     `json:"schedule"`
	Retention    int        `json:"retention"`
	Compression  bool       `json:"compression"`
	Encryption   bool       `json:"encryption"`
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	Name         string     `json:"name,omitempty"`
	Description  string     `json:"description,omitempty"`
	Sources      []string   `json:"sources,omitempty"`
	TargetType   TargetType `json:"target_type,omitempty"`
	TargetConfig string     `json:"target_config,omitempty"`
	Schedule     string     `json:"schedule,omitempty"`
	Retention    *int       `json:"retention,omitempty"`
	Compression  *bool      `json:"compression,omitempty"`
	Encryption   *bool      `json:"encryption,omitempty"`
	Enabled      *bool      `json:"enabled,omitempty"`
}

// ListTasksRequest 任务列表请求
type ListTasksRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Type     string `form:"type"`
	Enabled  *bool  `form:"enabled"`
}

// ListRecordsRequest 记录列表请求
type ListRecordsRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	TaskID   string `form:"task_id"`
	Status   string `form:"status"`
}
