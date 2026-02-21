// Package photos 照片管理模块
package photos

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"github.com/ruizi-store/rde/backend/modules/files"
	"go.uber.org/zap"
)

const (
	ModuleID      = "photos"
	ModuleName    = "相册"
	ModuleVersion = "1.0.0"
)

// Module 照片管理模块
type Module struct {
	ctx      *module.Context
	config   Config
	service  *Service
	scanner  *Scanner
	handler  *Handler
	thumbSvc *files.ThumbnailService
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
	return []string{"files", "users"}
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	// 加载配置
	m.config = DefaultConfig()
	if dir := ctx.Config.GetString("photos.thumbnail_cache_dir"); dir != "" {
		m.config.ThumbnailCacheDir = dir
	}
	if dir := ctx.Config.GetString("photos.originals_cache_dir"); dir != "" {
		m.config.OriginalsCacheDir = dir
	}
	if interval := ctx.Config.GetInt("photos.scan_interval"); interval > 0 {
		m.config.ScanInterval = interval
	}
	if workers := ctx.Config.GetInt("photos.max_scan_workers"); workers > 0 {
		m.config.MaxScanWorkers = workers
	}
	m.config.EnableAI = ctx.Config.GetBool("photos.enable_ai")
	if url := ctx.Config.GetString("photos.indexer_url"); url != "" {
		m.config.IndexerURL = url
	}

	// 获取 files 模块的缩略图服务
	filesModule := ctx.GetModule("files")
	if filesModule != nil {
		if fm, ok := filesModule.(*files.Module); ok {
			m.thumbSvc = fm.GetThumbnailService()
		}
	}

	// 如果没有获取到，创建独立实例
	if m.thumbSvc == nil {
		m.thumbSvc = files.NewThumbnailService(ctx.Logger, m.config.ThumbnailCacheDir)
	}

	// 自动迁移数据库
	if err := ctx.DB.AutoMigrate(&Photo{}, &Album{}, &AlbumPhoto{}, &Library{}); err != nil {
		return err
	}

	// 创建服务
	m.service = NewService(ctx.Logger, ctx.DB, m.thumbSvc)

	// 创建扫描器
	m.scanner = NewScanner(ctx.Logger, ctx.DB, m.service, m.config)

	// 创建处理器
	m.handler = NewHandler(m.service, m.scanner, m.config)

	ctx.Logger.Info("photos module initialized",
		zap.String("thumbnail_cache_dir", m.config.ThumbnailCacheDir),
		zap.Int("scan_interval", m.config.ScanInterval),
		zap.Bool("enable_ai", m.config.EnableAI))

	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	// 启动后台扫描任务
	m.scanner.Start()

	m.ctx.Logger.Info("photos module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	// 停止扫描器
	m.scanner.Stop()

	m.ctx.Logger.Info("photos module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	m.handler.RegisterRoutes(r)
}

// GetService 获取服务实例
func (m *Module) GetService() *Service {
	return m.service
}
