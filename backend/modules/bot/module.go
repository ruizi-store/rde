// Package bot 数字员工（带电脑的 AI 同事）模块
package bot

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"github.com/ruizi-store/rde/backend/modules/ai"
	"github.com/ruizi-store/rde/backend/modules/docker"
	"go.uber.org/zap"
)

const (
	ModuleID      = "bot"
	ModuleName    = "Bot Workbench"
	ModuleVersion = "1.2.0"
)

// Module bot 模块（实现 module.Module 接口）
type Module struct {
	ctx        *module.Context
	service    *Service
	handler    *Handler
	proxy      *StreamProxy
	skills     *SkillService
	memory     *MemoryService
	schedules  *ScheduleService
	approvals  *ApprovalService
	aiMod      *ai.Module
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

// ID 返回模块ID
func (m *Module) ID() string { return ModuleID }

// Name 返回模块名称
func (m *Module) Name() string { return ModuleName }

// Version 返回模块版本
func (m *Module) Version() string { return ModuleVersion }

// Dependencies 返回依赖模块
func (m *Module) Dependencies() []string {
	return []string{"users", "docker"}
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	if err := ctx.DB.AutoMigrate(&Bot{}, &BotMember{}, &BotMemory{}, &BotSchedule{}, &BotApproval{}); err != nil {
		return err
	}

	dataDir := ctx.Config.GetString("data_dir")
	if dataDir == "" {
		dataDir = "/var/lib/rde"
	}

	// 复用 docker 模块的 Service（沙箱容器管理）
	var dockerAPI DockerAPI
	if dm := ctx.GetModule("docker"); dm != nil {
		if dmod, ok := dm.(*docker.Module); ok {
			dockerAPI = dmod.GetService()
		}
	}
	if dockerAPI == nil {
		ctx.Logger.Warn("docker module unavailable, bot sandbox disabled (degraded mode)")
	}

	m.service = NewService(ctx.DB, ctx.Logger, dataDir, dockerAPI)
	m.skills = NewSkillService(ctx.Logger, dataDir)
	m.memory = NewMemoryService(ctx.DB)
	m.schedules = NewScheduleService(ctx.DB, ctx.Logger, m.service)
	m.approvals = NewApprovalService(ctx.DB, m.service, ctx.Logger)
	m.proxy = NewStreamProxy(m.service, ctx.Logger)
	m.handler = NewHandler(m.service, m.proxy, m.skills, m.memory, m.schedules, m.approvals)

	// P3/P4：向 ai 模块注入桥接（依赖方向 bot → ai；拓扑序 ai 先初始化）
	if am := ctx.GetModule("ai"); am != nil {
		if aiMod, ok := am.(*ai.Module); ok {
			m.aiMod = aiMod
			aiMod.SetBotBridge(
				&botSandbox{service: m.service, skills: m.skills, logger: ctx.Logger},
				&botChannelResolver{service: m.service},
				m.skills,
				&botMemoryProvider{svc: m.memory},
			)
			// P4：调度服务注入 ai 任务执行与渠道推送
			m.schedules.SetAIRunner(aiMod.RunBotTask, aiMod.SendToChannel)
			// P4：审批服务注入（工具执行 + /approve /deny 命令路由）
			m.approvals.SetNotifier(aiMod.SendToChannel)
			aiMod.SetApprovalManager(m.approvals)
		}
	}

	ctx.Logger.Info("bot module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	// 状态对账：进程重启后把数据库中的 running 修正为容器实际状态
	if m.service.docker != nil {
		if bots, err := m.service.List(context.Background()); err == nil {
			_ = bots // List 内部已做 syncStatus 对账
		}
	}
	// 启动定时调度（Bot 主动汇报）
	if err := m.schedules.Start(); err != nil {
		m.ctx.Logger.Warn("failed to start schedule service", zap.Error(err))
	}
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.schedules.Stop()
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	m.handler.RegisterRoutes(router)
}

// GetService 获取服务实例（供其他模块调用）
func (m *Module) GetService() *Service {
	return m.service
}
