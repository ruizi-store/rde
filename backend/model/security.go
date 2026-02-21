package model

import (
	"net"
	"time"
)

// SecurityConfig 安全配置
type SecurityConfig struct {
	ID string `json:"id" gorm:"primaryKey;size:36"`
	// 登录失败锁定
	MaxLoginAttempts int           `json:"max_login_attempts" gorm:"default:5"`         // 最大登录尝试次数
	LockDuration     time.Duration `json:"lock_duration" gorm:"-"`                      // 锁定时长
	LockDurationMins int           `json:"lock_duration_mins" gorm:"default:30"`        // 锁定时长（分钟）
	// 密码策略
	PasswordMinLength      int  `json:"password_min_length" gorm:"default:8"`          // 最小密码长度
	PasswordRequireUpper   bool `json:"password_require_upper" gorm:"default:true"`    // 需要大写字母
	PasswordRequireLower   bool `json:"password_require_lower" gorm:"default:true"`    // 需要小写字母
	PasswordRequireNumber  bool `json:"password_require_number" gorm:"default:true"`   // 需要数字
	PasswordRequireSpecial bool `json:"password_require_special" gorm:"default:false"` // 需要特殊字符
	PasswordExpireDays     int  `json:"password_expire_days" gorm:"default:0"`         // 密码过期天数，0=永不过期
	// 会话
	SessionTimeout      int  `json:"session_timeout" gorm:"default:1440"`        // 会话超时（分钟），默认24小时
	MaxConcurrentSessions int `json:"max_concurrent_sessions" gorm:"default:5"`  // 最大并发会话数
	// IP 限制
	EnableIPWhitelist bool   `json:"enable_ip_whitelist" gorm:"default:false"` // 启用 IP 白名单
	EnableIPBlacklist bool   `json:"enable_ip_blacklist" gorm:"default:false"` // 启用 IP 黑名单
	IPWhitelist       string `json:"ip_whitelist" gorm:"type:text"`            // IP 白名单（JSON 数组）
	IPBlacklist       string `json:"ip_blacklist" gorm:"type:text"`            // IP 黑名单（JSON 数组）
	// 2FA
	Require2FA bool `json:"require_2fa" gorm:"default:false"` // 强制启用 2FA
	// 远程访问服务（默认关闭）
	SSHEnabled      bool `json:"ssh_enabled" gorm:"default:false"`      // SSH 服务启用
	TerminalEnabled bool `json:"terminal_enabled" gorm:"default:false"` // Web 终端启用
	// 时间戳
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 表名
func (SecurityConfig) TableName() string {
	return "security_config"
}

// GetLockDuration 获取锁定时长
func (c *SecurityConfig) GetLockDuration() time.Duration {
	if c.LockDuration > 0 {
		return c.LockDuration
	}
	return time.Duration(c.LockDurationMins) * time.Minute
}

// IPAccessRule IP 访问规则
type IPAccessRule struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	Type        string    `json:"type" gorm:"size:10;not null"` // whitelist, blacklist
	IP          string    `json:"ip" gorm:"size:50;not null"`   // IP 地址或 CIDR
	Description string    `json:"description" gorm:"size:200"`
	CreatedBy   string    `json:"created_by" gorm:"size:36"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 表名
func (IPAccessRule) TableName() string {
	return "ip_access_rules"
}

// MatchIP 检查 IP 是否匹配规则
func (r *IPAccessRule) MatchIP(ip string) bool {
	// 精确匹配
	if r.IP == ip {
		return true
	}

	// CIDR 匹配
	_, cidr, err := net.ParseCIDR(r.IP)
	if err != nil {
		return false
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	return cidr.Contains(parsedIP)
}

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	UserID    string    `json:"user_id" gorm:"index;size:36"`
	Username  string    `json:"username" gorm:"index;size:50"`
	IP        string    `json:"ip" gorm:"index;size:50"`
	UserAgent string    `json:"user_agent" gorm:"size:500"`
	Success   bool      `json:"success" gorm:"default:false"`
	FailReason string   `json:"fail_reason" gorm:"size:200"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName 表名
func (LoginAttempt) TableName() string {
	return "login_attempts"
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	UserID      string    `json:"user_id" gorm:"index;size:36"`
	Username    string    `json:"username" gorm:"size:50"`
	EventType   string    `json:"event_type" gorm:"index;size:50;not null"` // login_failed, account_locked, suspicious_activity, etc.
	Severity    string    `json:"severity" gorm:"size:20;default:info"`     // info, warning, critical
	IP          string    `json:"ip" gorm:"size:50"`
	Description string    `json:"description" gorm:"size:500"`
	Details     string    `json:"details" gorm:"type:text"` // JSON 详细信息
	Resolved    bool      `json:"resolved" gorm:"default:false"`
	ResolvedBy  string    `json:"resolved_by" gorm:"size:36"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
}

// TableName 表名
func (SecurityEvent) TableName() string {
	return "security_events"
}

// SecurityEventType 安全事件类型
const (
	SecurityEventLoginFailed       = "login_failed"
	SecurityEventAccountLocked     = "account_locked"
	SecurityEventAccountUnlocked   = "account_unlocked"
	SecurityEventSuspiciousLogin   = "suspicious_login"
	SecurityEventPasswordChanged   = "password_changed"
	SecurityEventPasswordExpired   = "password_expired"
	SecurityEvent2FAEnabled        = "2fa_enabled"
	SecurityEvent2FADisabled       = "2fa_disabled"
	SecurityEventSessionRevoked    = "session_revoked"
	SecurityEventIPBlocked         = "ip_blocked"
	SecurityEventBruteForceAttempt = "brute_force_attempt"
)

// SecurityEventSeverity 安全事件严重程度
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// UpdateSecurityConfigRequest 更新安全配置请求
type UpdateSecurityConfigRequest struct {
	MaxLoginAttempts       *int  `json:"max_login_attempts"`
	LockDurationMins       *int  `json:"lock_duration_mins"`
	PasswordMinLength      *int  `json:"password_min_length"`
	PasswordRequireUpper   *bool `json:"password_require_upper"`
	PasswordRequireLower   *bool `json:"password_require_lower"`
	PasswordRequireNumber  *bool `json:"password_require_number"`
	PasswordRequireSpecial *bool `json:"password_require_special"`
	PasswordExpireDays     *int  `json:"password_expire_days"`
	SessionTimeout         *int  `json:"session_timeout"`
	MaxConcurrentSessions  *int  `json:"max_concurrent_sessions"`
	EnableIPWhitelist      *bool `json:"enable_ip_whitelist"`
	EnableIPBlacklist      *bool `json:"enable_ip_blacklist"`
	Require2FA             *bool `json:"require_2fa"`
}

// CreateIPRuleRequest 创建 IP 规则请求
type CreateIPRuleRequest struct {
	Type        string `json:"type" validate:"required,oneof=whitelist blacklist"`
	IP          string `json:"ip" validate:"required"`
	Description string `json:"description"`
}

// SecurityStatsResponse 安全统计响应
type SecurityStatsResponse struct {
	TotalLoginAttempts    int64              `json:"total_login_attempts"`
	FailedLoginAttempts   int64              `json:"failed_login_attempts"`
	LockedAccounts        int64              `json:"locked_accounts"`
	ActiveSessions        int64              `json:"active_sessions"`
	TwoFactorEnabledUsers int64              `json:"two_factor_enabled_users"`
	RecentEvents          []*SecurityEvent   `json:"recent_events"`
	LoginAttemptsByDay    map[string]int64   `json:"login_attempts_by_day"`
	TopFailedIPs          []IPFailedCount    `json:"top_failed_ips"`
}

// IPFailedCount IP 失败次数统计
type IPFailedCount struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

// PasswordValidation 密码验证结果
type PasswordValidation struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
}

// RemoteAccessSettings 远程访问设置
type RemoteAccessSettings struct {
	SSHEnabled      bool `json:"ssh_enabled"`
	SSHRunning      bool `json:"ssh_running"`
	SSHPort         int  `json:"ssh_port"`
	TerminalEnabled bool `json:"terminal_enabled"`
}

// UpdateRemoteAccessRequest 更新远程访问设置请求
type UpdateRemoteAccessRequest struct {
	SSHEnabled      *bool  `json:"ssh_enabled"`
	TerminalEnabled *bool  `json:"terminal_enabled"`
	Password        string `json:"password" binding:"required"` // 二次确认密码
}
