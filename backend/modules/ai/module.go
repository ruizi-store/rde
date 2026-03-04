// Package ai AI 聊天模块
package ai

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "ai"
	ModuleName    = "AI Assistant"
	ModuleVersion = "1.0.0"
)

// Module AI 模块（实现 module.Module 接口）
type Module struct {
	ctx      *module.Context
	service  *Service
	handler  *Handler
	skills   *SkillsService
	setup    *SetupService
	gateway  *GatewayService
	alerts   *AlertService
	sessions *SessionStore
	voice    *VoiceService
	coreAPI  *CoreAPI
	stopCh   chan struct{}
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
	logger := ctx.Logger

	// 获取数据目录
	dataDir := ctx.Config.GetString("data_dir")
	if dataDir == "" {
		dataDir = "/var/lib/rde"
	}
	aiDir := filepath.Join(dataDir, "ai")

	m.service = NewService(logger, aiDir)
	m.skills = NewSkillsService(logger, aiDir)
	m.service.SetSkills(m.skills)

	m.coreAPI = NewCoreAPI(logger)
	m.setup = NewSetupService(logger, aiDir, m.coreAPI)
	m.gateway = NewGatewayService(logger, aiDir, m.service, m.skills)
	m.alerts = NewAlertService(logger, aiDir, m.gateway)
	m.sessions = NewSessionStore(logger, aiDir)

	m.voice = NewVoiceService(logger, aiDir)

	m.handler = NewHandler(m.service, m.skills, m.setup, m.gateway, m.alerts, m.sessions, m.voice)
	m.stopCh = make(chan struct{})

	ctx.Logger.Info("ai module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.gateway.Start()
	m.alerts.Start()
	m.sessions.StartAutoSave(m.stopCh)
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	close(m.stopCh)
	m.alerts.Stop()
	m.gateway.Stop()
	m.sessions.Save()
	m.service.FlushSave()
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	m.handler.RegisterRoutes(router)
}
