package linuxlab

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "linuxlab"
	ModuleName    = "Linux Lab"
	ModuleVersion = "2.0.0"
)

type Module struct {
	ctx     *module.Context
	service *Service
}

func New() *Module {
	return &Module{}
}

func (m *Module) ID() string             { return ModuleID }
func (m *Module) Name() string           { return ModuleName }
func (m *Module) Version() string        { return ModuleVersion }
func (m *Module) Dependencies() []string { return nil }

func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	ctx.Logger.Info("Initializing Linux Lab module (Docker mode)")

	m.service = NewService(ctx.Logger)

	ctx.Logger.Info("Linux Lab module initialized",
		zap.Bool("docker_ok", m.service.DockerOK()),
		zap.Bool("image_ready", m.service.ImageExists()),
		zap.Bool("container_running", m.service.ContainerRunning()),
	)
	return nil
}

func (m *Module) Start() error {
	m.ctx.Logger.Info("Linux Lab module started (Docker mode)",
		zap.Bool("container_running", m.service.ContainerRunning()),
	)
	return nil
}

func (m *Module) Stop() error {
	// 不自动停止容器 — 它设置了 unless-stopped 策略
	m.ctx.Logger.Info("Linux Lab module stopped")
	return nil
}

func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	handler := NewHandler(m.service, m.ctx.Logger)

	rg := router.Group("/linuxlab")
	{
		rg.GET("/status", handler.GetStatus)
		rg.POST("/setup", handler.Setup)

		rg.GET("/boards", handler.ListBoards)
		rg.GET("/boards/:arch/:mach", handler.GetBoardDetail)
		rg.POST("/boards/switch", handler.SwitchBoard)

		rg.POST("/build", handler.Build)
		rg.GET("/build/status", handler.GetBuildStatus)

		rg.POST("/boot", handler.Boot)
		rg.DELETE("/boot", handler.StopBoot)

		rg.POST("/make", handler.ExecMakeTarget)
	}
}

func (m *Module) GetService() *Service {
	return m.service
}
