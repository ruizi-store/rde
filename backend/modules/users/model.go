package users

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID        string         `json:"id" gorm:"primaryKey;size:36"`
	Username  string         `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Email     string         `json:"email,omitempty" gorm:"size:128"`
	Password  string         `json:"-" gorm:"size:128;not null"` // 不返回给前端
	Nickname  string         `json:"nickname" gorm:"size:64"`
	Avatar    string         `json:"avatar,omitempty" gorm:"size:255"`
	Role      string         `json:"role" gorm:"size:32;default:'user'"` // admin, user, guest
	Status    string         `json:"status" gorm:"size:32;default:'active'"` // active, disabled
	GroupID   string         `json:"group_id,omitempty" gorm:"size:36;index"`
	Settings  string         `json:"settings,omitempty" gorm:"type:text"` // JSON 存储用户设置
	LastLogin *time.Time     `json:"last_login,omitempty"`
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联
	Group *UserGroup `json:"group,omitempty" gorm:"foreignKey:GroupID"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users_accounts"
}

// UserGroup 用户组模型
type UserGroup struct {
	ID          string         `json:"id" gorm:"primaryKey;size:36"`
	Name        string         `json:"name" gorm:"uniqueIndex;size:64;not null"`
	Description string         `json:"description,omitempty" gorm:"size:255"`
	Permissions string         `json:"permissions,omitempty" gorm:"type:text"` // JSON 存储权限列表
	CreatedAt   time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联
	Users []User `json:"users,omitempty" gorm:"foreignKey:GroupID"`
}

// TableName 指定表名
func (UserGroup) TableName() string {
	return "users_groups"
}

// LoginRequest 登录请求
// LoginRequest 登录请求
type LoginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DeviceToken string `json:"device_token,omitempty"` // 受信任设备令牌（可跳过 2FA）
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      *User     `json:"user"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email,omitempty" binding:"omitempty,email"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
	GroupID  string `json:"group_id,omitempty"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Email    string `json:"email,omitempty" binding:"omitempty,email"`
	Nickname string `json:"nickname,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Role     string `json:"role,omitempty"`
	Status   string `json:"status,omitempty"`
	GroupID  string `json:"group_id,omitempty"`
	Settings string `json:"settings,omitempty"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ResetPasswordRequest 管理员重置密码请求（无需旧密码）
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// CreateGroupRequest 创建用户组请求
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=64"`
	Description string `json:"description,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

// UpdateGroupRequest 更新用户组请求
type UpdateGroupRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

// UserRole 用户角色常量
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
	RoleGuest = "guest"
)

// UserStatus 用户状态常量
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// TwoFactorSettings 2FA 设置（存储在 User.Settings JSON 中）
type TwoFactorSettings struct {
	TotpSecret     string          `json:"totp_secret"`
	TotpEnabled    bool            `json:"totp_enabled"`
	BackupCodes    []string        `json:"backup_codes"`
	TrustedDevices []TrustedDevice `json:"trusted_devices,omitempty"`
}

// TrustedDevice 受信任设备
type TrustedDevice struct {
	ID        string    `json:"id"`         // 设备唯一标识（随机生成）
	Token     string    `json:"token"`      // 设备信任令牌
	UserAgent string    `json:"user_agent"` // 浏览器 UA
	IP        string    `json:"ip"`         // 登录 IP
	CreatedAt time.Time `json:"created_at"` // 创建时间
	ExpiresAt time.Time `json:"expires_at"` // 过期时间
}

// TwoFactorSetupResponse 2FA 设置响应
type TwoFactorSetupResponse struct {
	Secret      string   `json:"secret"`
	QRCodeURL   string   `json:"qr_code_url"`
	BackupCodes []string `json:"backup_codes"`
}

// Verify2FARequest 验证 2FA 请求
type Verify2FARequest struct {
	Code           string `json:"code" binding:"required"`
	TempToken      string `json:"temp_token,omitempty"`      // 登录时使用
	RememberDevice bool   `json:"remember_device,omitempty"` // 记住此设备
}
