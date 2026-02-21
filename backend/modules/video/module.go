// Package video 提供视频流媒体服务
package video

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "video"
	ModuleName    = "Video Player"
	ModuleVersion = "1.0.0"
)

// Module 视频模块
type Module struct {
	ctx     *module.Context
	service *Service
	handler *Handler
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
	return []string{"files"} // 依赖文件模块
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	// 从配置获取缓存目录
	cacheDir := ctx.Config.GetString("video.cache_dir")
	if cacheDir == "" {
		cacheDir = "/tmp/rde-video-cache"
	}

	// 缩略图目录
	thumbnailDir := ctx.Config.GetString("video.thumbnail_dir")
	if thumbnailDir == "" {
		homeDir, _ := os.UserHomeDir()
		thumbnailDir = filepath.Join(homeDir, ".cache", "rde", "video-thumbnails")
	}

	// HLS 配置
	hlsSegmentDuration := ctx.Config.GetInt("video.hls_segment_duration")
	if hlsSegmentDuration == 0 {
		hlsSegmentDuration = 4
	}

	maxConcurrentJobs := ctx.Config.GetInt("video.max_concurrent_jobs")
	if maxConcurrentJobs == 0 {
		maxConcurrentJobs = 3
	}

	// 创建服务
	m.service = NewService(ctx.Logger, ServiceConfig{
		CacheDir:           cacheDir,
		ThumbnailDir:       thumbnailDir,
		HLSSegmentDuration: hlsSegmentDuration,
		MaxConcurrentJobs:  maxConcurrentJobs,
	})

	// 创建处理器
	m.handler = NewHandler(m.service, ctx.Logger)

	ctx.Logger.Info("video module initialized",
		zap.String("cache_dir", cacheDir),
		zap.String("thumbnail_dir", thumbnailDir),
		zap.Int("hls_segment_duration", hlsSegmentDuration),
		zap.Int("max_concurrent_jobs", maxConcurrentJobs))

	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	// 清理过期的转码缓存
	go m.service.CleanupCache()

	m.ctx.Logger.Info("video module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	// 停止所有转码进程
	m.service.StopAllTranscodes()

	m.ctx.Logger.Info("video module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.RegisterRoutes(group)
}
