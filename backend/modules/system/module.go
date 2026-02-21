// Package system 系统模块
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

// Module 系统模块实现
type Module struct {
	service *Service
	handler *Handler
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

// ID 返回模块唯一标识
func (m *Module) ID() string {
	return "system"
}

// Name 返回模块名称
func (m *Module) Name() string {
	return "System"
}

// Version 返回模块版本
func (m *Module) Version() string {
	return "1.0.0"
}

// Dependencies 返回依赖的模块
func (m *Module) Dependencies() []string {
	return []string{} // 系统模块无依赖
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	// 系统模块不需要数据库
	m.service = NewService(ctx.Logger, "1.0.0", ctx.Config.GetString("data_path"))
	m.service.SetEventBus(ctx.EventBus)
	m.handler = NewHandler(m.service)
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	m.handler.RegisterRoutes(router)
}

// GetService 获取服务实例
func (m *Module) GetService() *Service {
	return m.service
}

// NewModule 创建系统模块（别名）
func NewModule() *Module {
	return New()
}
