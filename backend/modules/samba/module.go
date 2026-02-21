// Package samba 提供 Samba 文件共享管理模块
package samba

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "samba"
	ModuleName    = "Samba Manager"
	ModuleVersion = "1.0.0"
)

// Module Samba 管理模块
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
	return nil
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	// 创建服务
	m.service = NewService(ctx.Logger)

	// 创建处理器
	m.handler = NewHandler(m.service)

	ctx.Logger.Info("samba module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.ctx.Logger.Info("samba module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.ctx.Logger.Info("samba module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	samba := group.Group("/samba")
	m.handler.RegisterRoutes(samba)
}

// GetService 获取服务实例
func (m *Module) GetService() *Service {
	return m.service
}
