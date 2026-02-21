// Package sudo 提供特权操作执行模块
// 通过白名单机制安全地执行需要 sudo 权限的操作
package sudo

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "sudo"
	ModuleName    = "特权执行"
	ModuleVersion = "1.0.0"
)

// Module sudo 模块
type Module struct {
	logger   *zap.Logger
	db       *gorm.DB
	executor *Executor
	handler  *Handler
}

// NewModule 创建模块实例
func NewModule() module.Module {
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

// Dependencies 返回依赖模块
func (m *Module) Dependencies() []string {
	return []string{"users"} // 需要用户认证
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.logger = ctx.Logger.Named("sudo")
	m.db = ctx.DB

	// 自动迁移审计日志表
	if err := m.db.AutoMigrate(&AuditLog{}); err != nil {
		m.logger.Error("Failed to migrate audit log table", zap.Error(err))
		return err
	}

	// 创建执行器
	m.executor = NewExecutor(m.logger, m.db)

	// 创建处理器
	m.handler = NewHandler(m.executor, m.logger)

	m.logger.Info("Sudo module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.logger.Info("Sudo module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.logger.Info("Sudo module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.RegisterRoutes(group)
}

// GetExecutor 获取执行器（供其他模块调用）
func (m *Module) GetExecutor() *Executor {
	return m.executor
}
