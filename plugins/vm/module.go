// Package vm 虚拟机管理模块
package vm

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde-plugin-common/go/sdk"
)

// Module VM 模块（实现 sdk.PluginModule 接口）
type Module struct {
	service *Service
	handler *Handler
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "vm" }

// Init 初始化模块
func (m *Module) Init(ctx *sdk.PluginContext) error {
	dataDir := filepath.Join(ctx.DataDir, "vm")
	m.service = NewService(ctx.Logger, dataDir)
	m.handler = NewHandler(m.service)
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
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.RegisterRoutes(group)
}
