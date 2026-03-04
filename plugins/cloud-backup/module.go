package cloud_backup

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde-plugin-common/go/sdk"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Module 云备份模块（实现 sdk.PluginModule 接口）
type Module struct {
	logger         *zap.Logger
	db             *gorm.DB
	dataDir        string
	restoreHandler *CloudRestoreHandler
}

// New 创建云备份模块
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "cloud_backup" }

// Init 初始化模块
func (m *Module) Init(ctx *sdk.PluginContext) error {
	m.logger = ctx.Logger
	m.db = ctx.DB
	m.dataDir = ctx.DataDir
	m.restoreHandler = NewCloudRestoreHandler(ctx.DB, ctx.DataDir, ctx.Logger)
	ctx.Logger.Info("Cloud backup module initialized")
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
