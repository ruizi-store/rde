// Package backup 提供系统备份和还原功能
package backup

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "backup"
	ModuleName    = "备份还原"
	ModuleVersion = "1.0.0"
)

// Module 备份还原模块
type Module struct {
	ctx           *module.Context
	service       *Service
	handler       *Handler
	scheduler     *Scheduler
	p2pService    *P2PMigrateService
	migrateHandler *MigrateHandler
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

	// 自动迁移数据库
	if err := AutoMigrate(ctx.DB); err != nil {
		return err
	}

	// 创建服务
	m.service = NewService(ctx.Logger, ctx.DB, dataDir)

	// 创建定时任务调度器
	m.scheduler = NewScheduler(ctx.Logger, m.service)

	// 创建处理器
	m.handler = NewHandler(m.service)

	// 创建 P2P 迁移服务和处理器
	m.p2pService = NewP2PMigrateService(m.service)
	m.migrateHandler = NewMigrateHandler(m.p2pService)

	ctx.Logger.Info("Backup module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	// 启动定时任务调度器
	if err := m.scheduler.Start(); err != nil {
		return err
	}

	// 设置通知回调
	m.service.SetNotifyCallback(m.sendNotification)

	m.ctx.Logger.Info("Backup module started")
	return nil
}

// sendNotification 发送通知（通过事件总线）
func (m *Module) sendNotification(title, content string, isError bool) {
	if m.ctx.EventBus == nil {
		return
	}

	eventType := "backup.completed"
	if isError {
		eventType = "backup.failed"
	}

	m.ctx.EventBus.Publish(eventType, map[string]interface{}{
		"title":   title,
		"content": content,
		"source":  "backup",
	})
}

// Stop 停止模块
func (m *Module) Stop() error {
	// 停止调度器
	m.scheduler.Stop()

	m.ctx.Logger.Info("Backup module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	m.handler.RegisterRoutes(router)
	m.migrateHandler.RegisterMigrateRoutes(router)
}
