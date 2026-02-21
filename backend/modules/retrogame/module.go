// Package retrogame 复古游戏模块
// 负责 EmulatorJS 的按需下载安装和 ROM 管理
package retrogame

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "retrogame"
	ModuleName    = "复古游戏"
	ModuleVersion = "1.0.0"
)

// Module 复古游戏模块
type Module struct {
	ctx     *module.Context
	service *Service
	dataDir string
}

// New 创建模块
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string           { return ModuleID }
func (m *Module) Name() string         { return ModuleName }
func (m *Module) Version() string      { return ModuleVersion }
func (m *Module) Dependencies() []string { return nil }

func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	ctx.Logger.Info("Initializing retrogame module")

	m.dataDir = ctx.Config.GetString("data_dir")
	if m.dataDir == "" {
		m.dataDir = "/var/lib/rde"
	}

	m.service = NewService(ctx.Logger, m.dataDir)

	// 确保 emulatorjs 目录存在（即使尚未安装）
	emulatorDir := m.service.GetEmulatorDir()
	os.MkdirAll(emulatorDir, 0755)

	return nil
}

func (m *Module) Start() error {
	installed := m.service.IsInstalled()
	m.ctx.Logger.Info("Retrogame module started",
		zap.Bool("emulatorjs_installed", installed),
		zap.String("emulatorjs_dir", m.service.GetEmulatorDir()),
	)
	return nil
}

func (m *Module) Stop() error {
	m.ctx.Logger.Info("Retrogame module stopped")
	return nil
}

func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	handler := NewHandler(m.service, m.ctx.Logger)

	rg := router.Group("/retrogame")
	{
		rg.GET("/status", handler.GetStatus)
		rg.POST("/setup", handler.Setup)
		rg.DELETE("/emulatorjs", handler.Uninstall)
		rg.GET("/scan-roms", handler.ScanRoms)
	}
}

// GetService 获取服务实例
func (m *Module) GetService() *Service {
	return m.service
}

// GetEmulatorDir 返回 EmulatorJS 目录路径（供 bootstrap 注册静态文件用）
func (m *Module) GetEmulatorDir() string {
	return filepath.Join(m.dataDir, "emulatorjs")
}
