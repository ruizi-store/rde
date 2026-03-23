// Package translate 翻译模块
package translate

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "translate"
	ModuleName    = "翻译"
	ModuleVersion = "1.0.0"
)

// Module 翻译模块
type Module struct {
	ctx     *module.Context
	service *Service
	handler *Handler
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

// ID 返回模块ID
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
	return []string{}
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	logger := ctx.Logger

	// 创建服务
	m.service = NewService(logger)

	// 从配置读取 LibreTranslate URL（如果有）
	serviceURL := ctx.Config.GetString("translate.service_url")
	if serviceURL != "" {
		m.service.SetServiceURL(serviceURL)
	}

	// 创建处理器
	m.handler = NewHandler(m.service)

	logger.Info("Translate module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	// 检查服务状态
	status := m.service.CheckStatus()
	if status.Available {
		m.ctx.Logger.Info("LibreTranslate service is available", 
			// zap.String("url", status.URL),
		)
	} else {
		m.ctx.Logger.Warn("LibreTranslate service is not available, translation will not work until service is started",
			// zap.String("url", status.URL),
			// zap.String("message", status.Message),
		)
	}
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.ctx.Logger.Info("Translate module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	m.handler.RegisterRoutes(r)
}

// GetService 获取翻译服务（供其他模块调用）
func (m *Module) GetService() *Service {
	return m.service
}
