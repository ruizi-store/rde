// Package bot 数字员工（带电脑的 AI 同事）模块
package bot

import "time"

// Bot 数字员工定义
type Bot struct {
	ID           string `json:"id" gorm:"primaryKey;size:36"`
	Name         string `json:"name" gorm:"uniqueIndex;size:64;not null"` // 名字
	Description  string `json:"description" gorm:"size:512"`              // 职责描述
	Model        string `json:"model" gorm:"size:128"`                    // 模型名
	ProviderID   string `json:"provider_id" gorm:"size:64"`               // AI 提供商 ID
	Image        string `json:"image" gorm:"size:255"`                    // 沙箱镜像
	Channels     string `json:"channels" gorm:"type:text"`                // 渠道绑定 (JSON, P3 网关对接)
	Status       string `json:"status" gorm:"size:16;default:'created'"`  // created/starting/running/stopped/error
	ContainerID  string `json:"container_id" gorm:"size:64"`
	ErrorMessage string `json:"error_message" gorm:"size:512"`
	Enabled      bool   `json:"enabled" gorm:"default:true"`
	// 接管互斥：非空表示有人正在接管（三级可见性中的"接管"级）
	TakenOverBy     string     `json:"taken_over_by" gorm:"size:36"`
	TakenOverAt     *time.Time `json:"taken_over_at,omitempty"`
	TakenOverByName string     `json:"taken_over_by_name" gorm:"-"` // 展示用（运行时填充）
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// 运行时字段（不入库）
	Members []BotMember `json:"members,omitempty" gorm:"-"`
	MyRole  string      `json:"my_role,omitempty" gorm:"-"` // 当前请求者的角色（运行时填充）
}

// TableName 指定表名
func (Bot) TableName() string { return "bots" }

// BotMember Bot 的成员可见性
type BotMember struct {
	ID     uint   `json:"id" gorm:"primaryKey"`
	BotID  string `json:"bot_id" gorm:"uniqueIndex:idx_bot_member;size:36;not null"`
	UserID string `json:"user_id" gorm:"uniqueIndex:idx_bot_member;size:36;not null"`
	Role   string `json:"role" gorm:"size:16;default:'member'"` // admin=接管, member=预览, viewer=状态
}

// TableName 指定表名
func (BotMember) TableName() string { return "bot_members" }

// 成员角色常量（三级可见性）
const (
	RoleMemberAdmin  = "admin"  // 接管：全屏操作后交还
	RoleMemberMember = "member" // 预览：侧栏围观串流
	RoleMemberViewer = "viewer" // 状态：仅看点亮状态
)

// 有效角色集合
var validRoles = map[string]bool{
	RoleMemberAdmin:  true,
	RoleMemberMember: true,
	RoleMemberViewer: true,
}

// Bot 状态常量
const (
	StatusCreated  = "created"
	StatusStarting = "starting"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusError    = "error"
)
