// Package sync 提供基于 TUS 协议的文件同步模块
package sync

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "sync"
	ModuleName    = "文件同步"
	ModuleVersion = "1.0.0"
	defaultDataDir = "/var/lib/rde/sync"
)

// Module 文件同步模块
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

	// 获取数据目录
	dataDir := ctx.Config.GetString("data_dir")
	if dataDir == "" {
		dataDir = "/var/lib/rde"
	}

	// 创建服务
	m.service = NewService(ctx.Logger, ctx.DB, dataDir)
	if err := m.service.Init(); err != nil {
		return err
	}

	// 创建处理器
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
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	m.handler.RegisterRoutes(r)
}
