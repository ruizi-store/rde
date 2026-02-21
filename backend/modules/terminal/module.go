// Package terminal 终端模块 - 提供 Web 终端功能
package terminal

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

// Module 终端模块实现
type Module struct {
	ctx     *module.Context
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
	return "terminal"
}

// Name 返回模块名称
func (m *Module) Name() string {
	return "Terminal"
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
	m.logger = ctx.Logger.Named("terminal")

	// 创建服务
	m.service = NewService(m.logger)

	// 创建处理器
	m.handler = NewHandler(m.service, m.logger)

	// 设置数据目录（用于读取终端启用配置）
	if dataPath := ctx.Config.GetString("data_path"); dataPath != "" {
		m.handler.SetDataPath(dataPath)
	}

	m.logger.Info("Terminal module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.logger.Info("Terminal module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	if m.service != nil {
		m.service.Stop()
	}
	m.logger.Info("Terminal module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	// 从 Extra 获取 TokenManager
	tokenManager, ok := m.ctx.Extra["tokenManager"].(*auth.TokenManager)
	if !ok {
		m.logger.Error("TokenManager not found in context, terminal routes will not be registered")
		return
	}

	m.handler.RegisterRoutes(router, tokenManager)
	m.logger.Info("Terminal routes registered")
}

// GetService 获取服务实例
func (m *Module) GetService() *Service {
	return m.service
}
