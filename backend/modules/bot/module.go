// Package bot 数字员工（带电脑的 AI 同事）模块
package bot

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"github.com/ruizi-store/rde/backend/modules/docker"
)

const (
	ModuleID      = "bot"
	ModuleName    = "Bot Workbench"
	ModuleVersion = "1.0.0"
)

// Module bot 模块（实现 module.Module 接口）
type Module struct {
	ctx     *module.Context
	service *Service
	handler *Handler
	proxy   *StreamProxy
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

	if err := ctx.DB.AutoMigrate(&Bot{}, &BotMember{}); err != nil {
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
	m.proxy = NewStreamProxy(m.service, ctx.Logger)
	m.handler = NewHandler(m.service, m.proxy)

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
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
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
