package model

import (
	"time"
)

// AuditAction 审计动作
type AuditAction string

const (
	// 用户相关
	AuditLogin          AuditAction = "login"
	AuditLogout         AuditAction = "logout"
	AuditLoginFailed    AuditAction = "login_failed"
	AuditPasswordChange AuditAction = "password_change"
	AuditEnable2FA      AuditAction = "enable_2fa"
	AuditDisable2FA     AuditAction = "disable_2fa"
	// 用户管理
	AuditUserCreate  AuditAction = "user_create"
	AuditUserUpdate  AuditAction = "user_update"
	AuditUserDelete  AuditAction = "user_delete"
	AuditUserDisable AuditAction = "user_disable"
	AuditUserEnable  AuditAction = "user_enable"
	// 文件操作
	AuditFileCreate   AuditAction = "file_create"
	AuditFileRead     AuditAction = "file_read"
	AuditFileUpdate   AuditAction = "file_update"
	AuditFileDelete   AuditAction = "file_delete"
	AuditFileMove     AuditAction = "file_move"
	AuditFileCopy     AuditAction = "file_copy"
	AuditFileDownload AuditAction = "file_download"
	AuditFileUpload   AuditAction = "file_upload"
	// 分享
	AuditShareCreate AuditAction = "share_create"
	AuditShareAccess AuditAction = "share_access"
	AuditShareDelete AuditAction = "share_delete"
	// 权限
	AuditPermissionGrant  AuditAction = "permission_grant"
	AuditPermissionRevoke AuditAction = "permission_revoke"
	// 系统
	AuditSystemConfig  AuditAction = "system_config"
	AuditSettingChange AuditAction = "setting_change"
	// 安全服务
	AuditSSHEnable       AuditAction = "ssh_enable"
	AuditSSHDisable      AuditAction = "ssh_disable"
	AuditTerminalEnable  AuditAction = "terminal_enable"
	AuditTerminalDisable AuditAction = "terminal_disable"
)

// AuditStatus 审计状态
type AuditStatus string

const (
	AuditStatusSuccess AuditStatus = "success"
	AuditStatusFailed  AuditStatus = "failed"
)

// AuditLog 审计日志
type AuditLog struct {
	ID        string      `json:"id" gorm:"primaryKey;size:36"`
	UserID    string      `json:"user_id" gorm:"index;size:36"`
	Username  string      `json:"username" gorm:"size:50"`
	Action    AuditAction `json:"action" gorm:"index;size:30;not null"`
	Resource  string      `json:"resource" gorm:"size:1000"` // 操作的资源路径/名称
	Details   string      `json:"details" gorm:"type:text"`  // JSON 详细信息
	IP        string      `json:"ip" gorm:"size:50"`
	UserAgent string      `json:"user_agent" gorm:"size:500"`
	Status    AuditStatus `json:"status" gorm:"size:20;default:success"`
	ErrorMsg  string      `json:"error_msg" gorm:"size:500"`
	Duration  int64       `json:"duration"` // 操作耗时（毫秒）
	CreatedAt time.Time   `json:"created_at" gorm:"index"`
}

// TableName 表名
func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditLogQuery 审计日志查询条件
type AuditLogQuery struct {
	UserID    string      `json:"user_id"`
	Username  string      `json:"username"`
	Action    AuditAction `json:"action"`
	Resource  string      `json:"resource"`
	Status    AuditStatus `json:"status"`
	IP        string      `json:"ip"`
	StartTime *time.Time  `json:"start_time"`
	EndTime   *time.Time  `json:"end_time"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
}

// AuditLogResponse 审计日志响应
type AuditLogResponse struct {
	Logs  []*AuditLog `json:"logs"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// AuditStats 审计统计
type AuditStats struct {
	TotalLogs       int64            `json:"total_logs"`
	SuccessCount    int64            `json:"success_count"`
	FailedCount     int64            `json:"failed_count"`
	ActionCounts    map[string]int64 `json:"action_counts"`
	TopUsers        []UserActionStat `json:"top_users"`
	RecentActivity  []*AuditLog      `json:"recent_activity"`
}

// UserActionStat 用户操作统计
type UserActionStat struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Count    int64  `json:"count"`
}
