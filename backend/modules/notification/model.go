// Package notification 通知模块模型
package notification

import (
	"time"
)

// ===================== 常量定义 =====================

// Category 通知类别
type Category string

const (
	CategorySystem   Category = "system"   // 系统通知
	CategorySecurity Category = "security" // 安全警报
	CategoryStorage  Category = "storage"  // 存储警告
	CategoryBackup   Category = "backup"   // 备份通知
	CategoryApp      Category = "app"      // 应用通知
	CategoryUpdate   Category = "update"   // 更新提醒
)

// AllCategories 所有类别
var AllCategories = []Category{
	CategorySystem,
	CategorySecurity,
	CategoryStorage,
	CategoryBackup,
	CategoryApp,
	CategoryUpdate,
}

// Severity 严重级别
type Severity string

const (
	SeverityInfo     Severity = "info"     // 信息
	SeverityWarning  Severity = "warning"  // 警告
	SeverityError    Severity = "error"    // 错误
	SeverityCritical Severity = "critical" // 紧急
)

// AllSeverities 所有严重级别
var AllSeverities = []Severity{
	SeverityInfo,
	SeverityWarning,
	SeverityError,
	SeverityCritical,
}

// ChannelType 推送渠道类型
type ChannelType string

const (
	ChannelEmail    ChannelType = "email"
	ChannelWebhook  ChannelType = "webhook"
	ChannelTelegram ChannelType = "telegram"
	ChannelWeChat   ChannelType = "wechat"
	ChannelBark     ChannelType = "bark"
	ChannelDingTalk ChannelType = "dingtalk"
)

// ===================== 数据库模型 =====================

// Notification 站内通知
type Notification struct {
	ID        string     `json:"id" gorm:"primaryKey;size:36"`
	UserID    string     `json:"user_id" gorm:"size:36;index"`       // 接收用户（空=全局广播）
	Category  Category   `json:"category" gorm:"size:32;index"`      // 类别
	Severity  Severity   `json:"severity" gorm:"size:16;index"`      // 严重级别
	Title     string     `json:"title" gorm:"size:255"`              // 标题
	Content   string     `json:"content" gorm:"type:text"`           // 内容
	Link      string     `json:"link" gorm:"size:512"`               // 点击跳转链接
	Icon      string     `json:"icon" gorm:"size:64"`                // 图标名称
	Source    string     `json:"source" gorm:"size:64"`              // 来源模块
	IsRead    bool       `json:"is_read" gorm:"default:false;index"` // 是否已读
	ReadAt    *time.Time `json:"read_at"`                            // 已读时间
	CreatedAt time.Time  `json:"created_at" gorm:"index"`            // 创建时间
	ExpiresAt *time.Time `json:"expires_at"`                         // 过期时间
}

// TableName 表名
func (Notification) TableName() string {
	return "notifications"
}

// NotificationSettings 通知设置
type NotificationSettings struct {
	ID               string    `json:"id" gorm:"primaryKey;size:36"`
	UserID           string    `json:"user_id" gorm:"size:36;uniqueIndex"` // 用户ID
	Enabled          bool      `json:"enabled" gorm:"default:true"`        // 启用通知
	DesktopNotify    bool      `json:"desktop_notify" gorm:"default:true"` // 桌面通知
	SoundEnabled     bool      `json:"sound_enabled" gorm:"default:true"`  // 提示音
	DndEnabled       bool      `json:"dnd_enabled" gorm:"default:false"`   // 免打扰
	DndFrom          string    `json:"dnd_from" gorm:"size:8"`             // 免打扰开始 "22:00"
	DndTo            string    `json:"dnd_to" gorm:"size:8"`               // 免打扰结束 "08:00"
	FilterCategories string    `json:"filter_categories" gorm:"type:text"` // 启用的类别 JSON
	FilterSeverities string    `json:"filter_severities" gorm:"type:text"` // 启用的级别 JSON
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName 表名
func (NotificationSettings) TableName() string {
	return "notification_settings"
}

// NotificationChannel 推送渠道
type NotificationChannel struct {
	ID          string      `json:"id" gorm:"primaryKey;size:36"`
	UserID      string      `json:"user_id" gorm:"size:36;index"` // 用户ID（空=系统级）
	Name        string      `json:"name" gorm:"size:64"`          // 渠道名称
	Type        ChannelType `json:"type" gorm:"size:32"`          // 渠道类型
	Enabled     bool        `json:"enabled" gorm:"default:true"`  // 是否启用
	Config      string      `json:"config" gorm:"type:text"`      // JSON 配置
	Description string      `json:"description" gorm:"size:255"`  // 描述
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// TableName 表名
func (NotificationChannel) TableName() string {
	return "notification_channels"
}

// NotificationRule 推送规则
type NotificationRule struct {
	ID         string     `json:"id" gorm:"primaryKey;size:36"`
	UserID     string     `json:"user_id" gorm:"size:36;index"` // 用户ID
	Name       string     `json:"name" gorm:"size:64"`          // 规则名称
	ChannelID  string     `json:"channel_id" gorm:"size:36"`    // 关联渠道
	Categories string     `json:"categories" gorm:"type:text"`  // 触发类别 JSON
	Severities string     `json:"severities" gorm:"type:text"`  // 触发级别 JSON
	Enabled    bool       `json:"enabled" gorm:"default:true"`  // 是否启用
	Cooldown   int        `json:"cooldown" gorm:"default:0"`    // 冷却时间（秒）
	LastSentAt *time.Time `json:"last_sent_at"`                 // 上次发送
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName 表名
func (NotificationRule) TableName() string {
	return "notification_rules"
}

// NotificationHistory 推送历史
type NotificationHistory struct {
	ID          string      `json:"id" gorm:"primaryKey;size:36"`
	ChannelID   string      `json:"channel_id" gorm:"size:36;index"`
	ChannelType ChannelType `json:"channel_type" gorm:"size:32"`
	Category    Category    `json:"category" gorm:"size:32"`
	Title       string      `json:"title" gorm:"size:255"`
	Content     string      `json:"content" gorm:"type:text"`
	Recipient   string      `json:"recipient" gorm:"size:255"` // 接收者
	Status      string      `json:"status" gorm:"size:20"`     // pending, sent, failed
	ErrorMsg    string      `json:"error_msg" gorm:"type:text"`
	SentAt      *time.Time  `json:"sent_at"`
	CreatedAt   time.Time   `json:"created_at" gorm:"index"`
}

// TableName 表名
func (NotificationHistory) TableName() string {
	return "notification_history"
}

// ===================== 渠道配置结构 =====================

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	SMTPUsername string   `json:"smtp_username"`
	SMTPPassword string   `json:"smtp_password"`
	UseTLS       bool     `json:"use_tls"`
	FromAddress  string   `json:"from_address"`
	FromName     string   `json:"from_name"`
	ToAddresses  []string `json:"to_addresses"`
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"` // POST, GET
	Headers     map[string]string `json:"headers"`
	ContentType string            `json:"content_type"`
	Template    string            `json:"template"`
}

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// WeChatConfig 企业微信配置
type WeChatConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// BarkConfig Bark 配置
type BarkConfig struct {
	ServerURL string `json:"server_url"`
	DeviceKey string `json:"device_key"`
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

// ===================== 请求结构 =====================

// ListNotificationsRequest 获取通知列表请求
type ListNotificationsRequest struct {
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
	Category  string `form:"category" json:"category"`     // 逗号分隔
	Severity  string `form:"severity" json:"severity"`     // 逗号分隔
	IsRead    *bool  `form:"is_read" json:"is_read"`
	StartDate string `form:"start_date" json:"start_date"` // 2006-01-02
	EndDate   string `form:"end_date" json:"end_date"`
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	Enabled          *bool    `json:"enabled"`
	DesktopNotify    *bool    `json:"desktop_notify"`
	SoundEnabled     *bool    `json:"sound_enabled"`
	DndEnabled       *bool    `json:"dnd_enabled"`
	DndFrom          string   `json:"dnd_from"`
	DndTo            string   `json:"dnd_to"`
	FilterCategories []string `json:"filter_categories"`
	FilterSeverities []string `json:"filter_severities"`
}

// CreateChannelRequest 创建渠道请求
type CreateChannelRequest struct {
	Name        string      `json:"name" binding:"required"`
	Type        ChannelType `json:"type" binding:"required"`
	Config      interface{} `json:"config" binding:"required"`
	Description string      `json:"description"`
}

// UpdateChannelRequest 更新渠道请求
type UpdateChannelRequest struct {
	Name        string      `json:"name"`
	Config      interface{} `json:"config"`
	Description string      `json:"description"`
	Enabled     *bool       `json:"enabled"`
}

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Name       string   `json:"name" binding:"required"`
	ChannelID  string   `json:"channel_id" binding:"required"`
	Categories []string `json:"categories"`
	Severities []string `json:"severities"`
	Cooldown   int      `json:"cooldown"`
}

// UpdateRuleRequest 更新规则请求
type UpdateRuleRequest struct {
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
	Severities []string `json:"severities"`
	Cooldown   *int     `json:"cooldown"`
	Enabled    *bool    `json:"enabled"`
}

// SendNotificationRequest 发送通知请求（内部API）
type SendNotificationRequest struct {
	UserID   string   `json:"user_id"`                       // 空=广播给所有用户
	Category Category `json:"category" binding:"required"`
	Severity Severity `json:"severity" binding:"required"`
	Title    string   `json:"title" binding:"required"`
	Content  string   `json:"content" binding:"required"`
	Link     string   `json:"link"`
	Source   string   `json:"source"`
}

// ===================== 响应结构 =====================

// NotificationListResponse 通知列表响应
type NotificationListResponse struct {
	Items       []*Notification `json:"items"`
	Total       int64           `json:"total"`
	Page        int             `json:"page"`
	PageSize    int             `json:"page_size"`
	UnreadCount int64           `json:"unread_count"`
}

// SettingsResponse 设置响应
type SettingsResponse struct {
	Enabled          bool     `json:"enabled"`
	DesktopNotify    bool     `json:"desktop_notify"`
	SoundEnabled     bool     `json:"sound_enabled"`
	DndEnabled       bool     `json:"dnd_enabled"`
	DndFrom          string   `json:"dnd_from"`
	DndTo            string   `json:"dnd_to"`
	FilterCategories []string `json:"filter_categories"`
	FilterSeverities []string `json:"filter_severities"`
}

// UnreadCountResponse 未读数量响应
type UnreadCountResponse struct {
	Count int64 `json:"count"`
}

// ChannelWithStatus 渠道及状态
type ChannelWithStatus struct {
	NotificationChannel
	IsConfigured bool `json:"is_configured"`
}
