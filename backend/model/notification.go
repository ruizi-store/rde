package model

import (
	"time"
)

// NotificationChannelType 通知渠道类型
type NotificationChannelType string

const (
	ChannelEmail    NotificationChannelType = "email"
	ChannelWebhook  NotificationChannelType = "webhook"
	ChannelTelegram NotificationChannelType = "telegram"
	ChannelWeChat   NotificationChannelType = "wechat"
	ChannelBark     NotificationChannelType = "bark"
	ChannelPushover NotificationChannelType = "pushover"
)

// NotificationEventType 通知事件类型
type NotificationEventType string

const (
	EventUserLogin       NotificationEventType = "user_login"
	EventUserLoginFailed NotificationEventType = "user_login_failed"
	EventAccountLocked   NotificationEventType = "account_locked"
	EventFileShared      NotificationEventType = "file_shared"
	EventShareAccessed   NotificationEventType = "share_accessed"
	EventBackupComplete  NotificationEventType = "backup_complete"
	EventBackupFailed    NotificationEventType = "backup_failed"
	EventStorageWarning  NotificationEventType = "storage_warning"
	EventStorageCritical NotificationEventType = "storage_critical"
	EventSystemUpdate    NotificationEventType = "system_update"
	EventSecurityAlert   NotificationEventType = "security_alert"
	EventCustom          NotificationEventType = "custom"
)

// NotificationChannel 通知渠道配置
type NotificationChannel struct {
	ID          string                  `json:"id" gorm:"primaryKey;size:36"`
	Name        string                  `json:"name" gorm:"size:100;not null"`
	Type        NotificationChannelType `json:"type" gorm:"size:20;not null"`
	Enabled     bool                    `json:"enabled" gorm:"default:false"`
	Config      string                  `json:"config" gorm:"type:text"` // JSON 配置
	Description string                  `json:"description" gorm:"size:500"`
	CreatedBy   string                  `json:"created_by" gorm:"size:36"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// TableName 表名
func (NotificationChannel) TableName() string {
	return "notification_channels"
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	UseTLS       bool   `json:"use_tls"`
	FromAddress  string `json:"from_address"`
	FromName     string `json:"from_name"`
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"` // POST, GET
	Headers     map[string]string `json:"headers"`
	ContentType string            `json:"content_type"` // application/json, application/x-www-form-urlencoded
	Template    string            `json:"template"`     // 消息模板
}

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// WeChatConfig 微信配置（企业微信机器人）
type WeChatConfig struct {
	WebhookURL string `json:"webhook_url"` // 企业微信机器人 Webhook URL
}

// BarkConfig Bark 配置 (iOS 推送)
type BarkConfig struct {
	ServerURL string `json:"server_url"`
	DeviceKey string `json:"device_key"`
}

// PushoverConfig Pushover 配置
type PushoverConfig struct {
	APIToken string `json:"api_token"`
	UserKey  string `json:"user_key"`
}

// NotificationRule 通知规则
type NotificationRule struct {
	ID          string                `json:"id" gorm:"primaryKey;size:36"`
	Name        string                `json:"name" gorm:"size:100;not null"`
	EventType   NotificationEventType `json:"event_type" gorm:"size:50;not null;index"`
	ChannelID   string                `json:"channel_id" gorm:"size:36;not null;index"`
	Enabled     bool                  `json:"enabled" gorm:"default:true"`
	Conditions  string                `json:"conditions" gorm:"type:text"` // JSON 条件
	Template    string                `json:"template" gorm:"type:text"`   // 自定义消息模板
	Cooldown    int                   `json:"cooldown" gorm:"default:0"`   // 冷却时间（秒），防止频繁通知
	LastSentAt  *time.Time            `json:"last_sent_at"`
	CreatedBy   string                `json:"created_by" gorm:"size:36"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// TableName 表名
func (NotificationRule) TableName() string {
	return "notification_rules"
}

// NotificationHistory 通知历史
type NotificationHistory struct {
	ID          string                `json:"id" gorm:"primaryKey;size:36"`
	RuleID      string                `json:"rule_id" gorm:"size:36;index"`
	ChannelID   string                `json:"channel_id" gorm:"size:36;index"`
	ChannelType NotificationChannelType `json:"channel_type" gorm:"size:20"`
	EventType   NotificationEventType `json:"event_type" gorm:"size:50;index"`
	Title       string                `json:"title" gorm:"size:200"`
	Content     string                `json:"content" gorm:"type:text"`
	Recipient   string                `json:"recipient" gorm:"size:200"` // 接收者
	Status      string                `json:"status" gorm:"size:20"`     // pending, sent, failed
	ErrorMsg    string                `json:"error_msg" gorm:"size:500"`
	SentAt      *time.Time            `json:"sent_at"`
	CreatedAt   time.Time             `json:"created_at" gorm:"index"`
}

// TableName 表名
func (NotificationHistory) TableName() string {
	return "notification_history"
}

// NotificationTemplate 通知模板
type NotificationTemplate struct {
	ID          string                `json:"id" gorm:"primaryKey;size:36"`
	Name        string                `json:"name" gorm:"size:100;not null"`
	EventType   NotificationEventType `json:"event_type" gorm:"size:50;index"`
	Subject     string                `json:"subject" gorm:"size:200"`    // 主题（邮件用）
	Content     string                `json:"content" gorm:"type:text"`   // 内容模板
	ContentType string                `json:"content_type" gorm:"size:20"` // text, html, markdown
	IsDefault   bool                  `json:"is_default" gorm:"default:false"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// TableName 表名
func (NotificationTemplate) TableName() string {
	return "notification_templates"
}

// ===================== 请求/响应结构 =====================

// CreateChannelRequest 创建通知渠道请求
type CreateChannelRequest struct {
	Name        string                  `json:"name" validate:"required"`
	Type        NotificationChannelType `json:"type" validate:"required"`
	Config      interface{}             `json:"config" validate:"required"`
	Description string                  `json:"description"`
}

// UpdateChannelRequest 更新通知渠道请求
type UpdateChannelRequest struct {
	Name        string      `json:"name"`
	Config      interface{} `json:"config"`
	Description string      `json:"description"`
	Enabled     *bool       `json:"enabled"`
}

// CreateRuleRequest 创建通知规则请求
type CreateRuleRequest struct {
	Name       string                `json:"name" validate:"required"`
	EventType  NotificationEventType `json:"event_type" validate:"required"`
	ChannelID  string                `json:"channel_id" validate:"required"`
	Conditions map[string]interface{} `json:"conditions"`
	Template   string                `json:"template"`
	Cooldown   int                   `json:"cooldown"`
}

// UpdateRuleRequest 更新通知规则请求
type UpdateRuleRequest struct {
	Name       string                 `json:"name"`
	Conditions map[string]interface{} `json:"conditions"`
	Template   string                 `json:"template"`
	Cooldown   *int                   `json:"cooldown"`
	Enabled    *bool                  `json:"enabled"`
}

// SendNotificationRequest 发送通知请求
type SendNotificationRequest struct {
	ChannelID string                 `json:"channel_id" validate:"required"`
	Title     string                 `json:"title" validate:"required"`
	Content   string                 `json:"content" validate:"required"`
	Data      map[string]interface{} `json:"data"`
}

// TestChannelRequest 测试通知渠道请求
type TestChannelRequest struct {
	ChannelID string `json:"channel_id" validate:"required"`
}

// NotificationEvent 通知事件（用于触发通知）
type NotificationEvent struct {
	Type      NotificationEventType  `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	UserID    string                 `json:"user_id"`
	Timestamp time.Time              `json:"timestamp"`
}
