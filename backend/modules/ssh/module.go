// Package ssh SSH远程连接模块 - 提供SSH终端和SFTP功能
package ssh

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Module SSH模块实现
type Module struct {
	ctx     *module.Context
	db      *gorm.DB
	service *Service
	handler *Handler
	logger  *zap.Logger
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

// ID 返回模块唯一标识
func (m *Module) ID() string {
	return "ssh"
}

// Name 返回模块名称
func (m *Module) Name() string {
	return "SSH Remote"
}

// Version 返回模块版本
func (m *Module) Version() string {
	return "1.0.0"
}

// Dependencies 返回依赖的模块
func (m *Module) Dependencies() []string {
	return []string{"users"} // 依赖用户模块进行认证
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	m.db = ctx.DB
	m.logger = ctx.Logger.Named("ssh")

	// 自动迁移数据库表
	if err := m.db.AutoMigrate(&Connection{}); err != nil {
		return err
	}

	// 从配置获取加密密钥
	encryptionKey := ctx.Config.GetString("encryption_key")
	if encryptionKey == "" {
		encryptionKey = "rde-default-encryption-key-32b!" // 默认密钥，生产环境应配置
	}

	// 创建服务
	m.service = NewService(m.db, encryptionKey, m.logger)

	// 创建处理器
	m.handler = NewHandler(m.service, m.logger)

	m.logger.Info("SSH module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.logger.Info("SSH module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	if m.service != nil {
		m.service.Stop()
	}
	m.logger.Info("SSH module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	// 从 Extra 获取 TokenManager
	tokenManager, ok := m.ctx.Extra["tokenManager"].(*auth.TokenManager)
	if !ok {
		m.logger.Error("TokenManager not found in context, SSH routes will not be registered")
		return
	}

	m.handler.RegisterRoutes(router, tokenManager)
	m.logger.Info("SSH routes registered")
}

// GetService 获取服务实例
func (m *Module) GetService() *Service {
	return m.service
}
