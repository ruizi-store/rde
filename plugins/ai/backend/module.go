// Package ai AI 聊天模块
package ai

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde-plugin-common/go/sdk"
)

// Module AI 模块（实现 sdk.PluginModule 接口）
type Module struct {
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

func (m *Module) ID() string { return "ai" }

// Init 初始化模块
func (m *Module) Init(ctx *sdk.PluginContext) error {
	logger := ctx.Logger
	aiDir := filepath.Join(ctx.DataDir, "ai")

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
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.RegisterRoutes(group)
}
