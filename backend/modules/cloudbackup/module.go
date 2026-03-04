// Package cloudbackup 云备份模块
package cloudbackup

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	ModuleID      = "cloudbackup"
	ModuleName    = "Cloud Backup"
	ModuleVersion = "1.0.0"
)

// Module 云备份模块（实现 module.Module 接口）
type Module struct {
	ctx            *module.Context
	logger         *zap.Logger
	db             *gorm.DB
	dataDir        string
	restoreHandler *CloudRestoreHandler
}

// New 创建云备份模块
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
	return []string{"users", "backup"}
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	m.logger = ctx.Logger
	m.db = ctx.DB

	// 获取数据目录
	dataDir := ctx.Config.GetString("data_dir")
	if dataDir == "" {
		dataDir = "/var/lib/rde"
	}
	m.dataDir = dataDir

	m.restoreHandler = NewCloudRestoreHandler(ctx.DB, dataDir, ctx.Logger)
	ctx.Logger.Info("cloudbackup module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error { return nil }

// Stop 停止模块
func (m *Module) Stop() error { return nil }

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	g := router.Group("/cloud-backup")
	{
		// 云端备份目标 API
		g.POST("/test", m.handleTest)
		// 云端恢复 API
		g.POST("/login", m.restoreHandler.CloudLogin)
		g.POST("/email", m.restoreHandler.CloudSendEmail)
		g.POST("/backups", m.restoreHandler.CloudListBackups)
		g.POST("/restore", m.restoreHandler.CloudRestore)
		g.GET("/restore/status", m.restoreHandler.CloudRestoreStatus)
	}
}

// handleTest 测试云备份连接
func (m *Module) handleTest(c *gin.Context) {
	var req struct {
		Config string `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "缺少配置"})
		return
	}

	target := &CloudTarget{}
	if err := target.Configure(req.Config); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := target.Test()
	c.JSON(200, result)
}
