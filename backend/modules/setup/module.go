// Package setup 系统初始化向导模块
package setup

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "setup"
	ModuleName    = "初始化向导"
	ModuleVersion = "1.0.0"
)

// Module 初始化向导模块
type Module struct {
	ctx     *module.Context
	service *Service
}

// NewModule 创建模块实例
func NewModule() *Module {
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
	// Setup 模块是最基础的模块，不依赖其他模块
	return nil
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	ctx.Logger.Info("Initializing setup module")

	// 自动迁移数据库表
	if err := ctx.DB.AutoMigrate(&SetupSettings{}); err != nil {
		return err
	}

	// 手动添加可能缺失的列（用于从旧版本升级）
	migrator := ctx.DB.Migrator()
	if !migrator.HasColumn(&SetupSettings{}, "https_port") {
		if err := migrator.AddColumn(&SetupSettings{}, "https_port"); err != nil {
			ctx.Logger.Warn("Failed to add https_port column", zap.Error(err))
		}
	}
	if !migrator.HasColumn(&SetupSettings{}, "http_port") {
		if err := migrator.AddColumn(&SetupSettings{}, "http_port"); err != nil {
			ctx.Logger.Warn("Failed to add http_port column", zap.Error(err))
		}
	}

	// 获取数据目录配置
	dataDir := ctx.Config.GetString("data_dir")
	if dataDir == "" {
		dataDir = "/var/lib/rde"
	}

	// 创建服务
	m.service = NewService(ctx.DB, ctx.Logger, dataDir, ctx.Config)

	// 注入 bootstrap 的共享 TokenManager，确保 setup 完成时生成的
	// auto-login token 使用与全局认证中间件相同的 JWT 密钥
	if tokenManager, ok := ctx.Extra["tokenManager"].(*auth.TokenManager); ok {
		m.service.SetTokenManager(tokenManager)
	} else {
		ctx.Logger.Warn("TokenManager not found in Extra, auto-login after setup will not work")
	}

	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.ctx.Logger.Info("Setup module started",
		zap.Bool("completed", m.service.IsCompleted()),
	)
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.ctx.Logger.Info("Setup module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	handler := NewHandler(m.service, m.ctx.Logger)

	// Setup 路由不需要认证
	setup := router.Group("/setup")
	{
		// 获取状态
		setup.GET("/status", handler.GetStatus)

		// Step 1: 系统检查
		setup.GET("/check", handler.CheckSystem)
		setup.POST("/check/complete", handler.CompleteStep1)
		setup.POST("/install-deps", handler.InstallDeps)

		// Step 2: 语言时区
		setup.POST("/locale", handler.SetLocale)

		// Step 3: 创建用户
		setup.POST("/user", handler.CreateAdmin)
		setup.POST("/user/verify-2fa", handler.Verify2FA)

		// 安全配置（可选随机端口）
		setup.POST("/security", handler.ConfigureSecurity)

		// Step 4: 存储配置
		setup.GET("/drives", handler.GetDrives)
		setup.POST("/storage", handler.ConfigureStorage)
		setup.POST("/storage/skip", handler.SkipStorage)
		setup.GET("/storage/available-disks", handler.GetAvailableDisks)

		// Step 5: 网络设置
		setup.POST("/network", handler.ConfigureNetwork)
		setup.POST("/network/skip", handler.SkipNetwork)

		// Step 6: 功能选择
		setup.GET("/features", handler.GetFeatures)
		setup.POST("/features", handler.SaveFeatures)
		setup.POST("/features/skip", handler.SkipFeatures)

		// Step 7: 完成
		setup.POST("/complete", handler.Complete)
	}

	// 恢复出厂设置（需要认证）
	// 从 Extra 获取 TokenManager
	if tokenManager, ok := m.ctx.Extra["tokenManager"].(*auth.TokenManager); ok {
		setup.POST("/factory-reset", auth.Middleware(tokenManager), auth.RequireAdmin(), handler.FactoryReset)
	} else {
		m.ctx.Logger.Warn("TokenManager not found in Extra, factory-reset will not be available")
	}

	// 模块设置 API (需要认证，实际部署时应添加认证中间件)
	settings := router.Group("/settings")
	{
		settings.GET("/modules", handler.GetModuleSettings)
		settings.PUT("/modules/:id", handler.UpdateModuleSetting)
	}
}

// Service 返回服务实例（供其他模块使用）
func (m *Module) Service() *Service {
	return m.service
}

// IsCompleted 检查初始化是否完成
func (m *Module) IsCompleted() bool {
	if m.service == nil {
		return false
	}
	return m.service.IsCompleted()
}

// NeedsSetup 检查是否需要初始化
func (m *Module) NeedsSetup() bool {
	return !m.IsCompleted()
}

// MarkCompleted 标记安装为已完成（用于 CLI 初始化）
func (m *Module) MarkCompleted() error {
	if m.service == nil {
		return nil
	}
	return m.service.MarkSetupCompleted()
}
