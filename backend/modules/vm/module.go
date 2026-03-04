// Package vm 虚拟机管理模块
package vm

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "vm"
	ModuleName    = "Virtual Machine"
	ModuleVersion = "1.0.0"
)

// Module VM 模块（实现 module.Module 接口）
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
	return []string{"users"}
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	// 获取数据目录
	dataDir := ctx.Config.GetString("data_dir")
	if dataDir == "" {
		dataDir = "/var/lib/rde"
	}
	vmDataDir := filepath.Join(dataDir, "vm")

	m.service = NewService(ctx.Logger, vmDataDir)
	m.handler = NewHandler(m.service)

	ctx.Logger.Info("vm module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	if m.service != nil {
		return m.service.Start()
	}
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	if m.service != nil {
		m.service.Stop()
	}
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	m.handler.RegisterRoutes(router)
}
