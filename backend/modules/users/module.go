// Package users 用户管理模块
package users

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "users"
	ModuleName    = "用户管理"
	ModuleVersion = "1.0.0"
)

// Module 用户管理模块
type Module struct {
	ctx          *module.Context
	service      *Service
	tokenManager *auth.TokenManager
	eventBus     module.EventBus
}

// New 创建用户模块
func New() *Module {
	return &Module{}
}

// ID 返回模块 ID
func (m *Module) ID() string {
	return ModuleID
}

// Name 返回模块名称
func (m *Module) Name() string {
	return ModuleName
}

// Version 返回模块版本
func (m *Module) Version() string {
	return ModuleVersion
}

// Dependencies 返回依赖的模块
func (m *Module) Dependencies() []string {
	// 用户模块是基础模块，不依赖其他模块
	return nil
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	m.eventBus = ctx.EventBus
	ctx.Logger.Info("Initializing users module")

	// 自动迁移数据库表
	if err := ctx.DB.AutoMigrate(&User{}, &UserGroup{}); err != nil {
		return err
	}

	// 从 Extra 获取 TokenManager
	if tm, ok := ctx.Extra["tokenManager"].(*auth.TokenManager); ok {
		m.tokenManager = tm
		ctx.Logger.Info("TokenManager loaded successfully")
	} else {
		ctx.Logger.Warn("TokenManager not found in context, JWT features will be disabled")
	}

	// 创建服务
	m.service = NewService(ctx.DB, ctx.Logger)
	m.service.SetEventBus(ctx.EventBus)

	// 设置头像存储目录
	avatarsDir := filepath.Join(ctx.Config.GetString("data_dir"), "avatars")
	m.service.SetAvatarsDir(avatarsDir)

	// 确保存在管理员账户
	if err := m.service.EnsureAdminExists(); err != nil {
		ctx.Logger.Error("Failed to ensure admin exists", zap.Error(err))
	}

	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.ctx.Logger.Info("Users module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.ctx.Logger.Info("Users module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	handler := NewHandler(m.service, m.tokenManager, m.ctx.Logger)

	// 公开路由（无需认证）
	public := router.Group("/auth")
	{
		public.POST("/login", handler.Login)
		public.POST("/register", handler.Register) // 可选，根据配置决定是否开放
		public.POST("/refresh", handler.RefreshToken) // 刷新令牌
		public.POST("/verify-2fa", handler.Verify2FA) // 两步验证
	}

	// 需要认证的路由（已由全局 auth 中间件保护）
	users := router.Group("/users")
	{
		users.GET("", handler.ListUsers)
		users.GET("/current", handler.GetCurrentUser)
		users.GET("/:id", handler.GetUser)
		users.POST("", handler.CreateUser)
		users.PUT("/:id", handler.UpdateUser)
		users.DELETE("/:id", handler.DeleteUser)
		users.PUT("/:id/password", handler.ChangePassword)
		users.PUT("/:id/reset-password", handler.ResetPassword)
		users.POST("/:id/avatar", handler.UploadAvatar)

		// 两步验证管理
		users.GET("/2fa/status", handler.Get2FAStatus)
		users.POST("/2fa/setup", handler.Setup2FA)
		users.POST("/2fa/enable", handler.Enable2FA)
		users.DELETE("/2fa", handler.Disable2FA)
	}

	// 用户组
	groups := router.Group("/groups")
	{
		groups.GET("", handler.ListGroups)
		groups.POST("", handler.CreateGroup)
		groups.PUT("/:id", handler.UpdateGroup)
		groups.DELETE("/:id", handler.DeleteGroup)
	}
}

// GetService 获取服务实例（供其他模块调用）
func (m *Module) GetService() *Service {
	return m.service
}

// CreateAdminUser 创建管理员用户（用于安装初始化）
func (m *Module) CreateAdminUser(username, password string) error {
	return m.service.CreateUser(&CreateUserRequest{
		Username: username,
		Password: password,
		Role:     RoleAdmin,
	})
}

// NewModule 创建用户模块（别名）
func NewModule() *Module {
	return New()
}
