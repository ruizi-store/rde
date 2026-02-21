// Package download 下载管理模块
package download

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

// Module 下载模块
type Module struct {
	service *Service
	handler *Handler
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string      { return "download" }
func (m *Module) Name() string    { return "Download Manager" }
func (m *Module) Version() string { return "1.0.0" }
func (m *Module) Dependencies() []string {
	return nil
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	dataDir := filepath.Join(ctx.Config.GetString("data_dir"), "download")

	// 默认下载目录：用户主目录/Downloads
	downloadDir := filepath.Join(ctx.Config.GetString("data_dir"), "downloads")
	if homeDir, err := os.UserHomeDir(); err == nil {
		downloadDir = filepath.Join(homeDir, "Downloads")
	}

	// 获取底层 sql.DB
	sqlDB, err := ctx.DB.DB()
	if err != nil {
		return err
	}

	// 使用带数据库的构造函数
	service, err := NewService(ctx.Logger, sqlDB, dataDir, downloadDir)
	if err != nil {
		return err
	}
	m.service = service
	m.handler = NewHandler(m.service)

	return nil
}

// Start 启动模块（不再同步启动 aria2c，改为首次使用时懒启动）
func (m *Module) Start() error {
	return nil
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
