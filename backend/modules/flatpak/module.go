// Package flatpak Flatpak 应用模块
package flatpak

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

// Module Flatpak 模块
type Module struct {
	service *Service
	handler *Handler
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string      { return "flatpak" }
func (m *Module) Name() string    { return "Flatpak Apps" }
func (m *Module) Version() string { return "1.0.0" }
func (m *Module) Dependencies() []string {
	return nil
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	dataDir := filepath.Join(ctx.Config.GetString("data_dir"), "flatpak")
	m.service = NewService(ctx.Logger, dataDir)
	m.handler = NewHandler(m.service)
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	return m.service.Start()
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.service.Stop()
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.RegisterRoutes(group)
}

// GetInstaller 获取安装管理器实例（供其他模块注入加速提供者）
func (m *Module) GetInstaller() *Installer {
	if m.service != nil {
		return m.service.installer
	}
	return nil
}
