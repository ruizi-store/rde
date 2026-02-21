// Package files 提供文件管理模块
package files

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/common"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "files"
	ModuleName    = "File Manager"
	ModuleVersion = "1.0.0"
)

// Module 文件管理模块
type Module struct {
	ctx        *module.Context
	service    *Service
	handler    *Handler
	thumbnails *ThumbnailService
}

// NewModule 创建模块实例
func NewModule() module.Module {
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
	return []string{"users"} // 需要用户认证
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	// 从配置获取根路径（允许访问的目录列表）
	rootPaths := ctx.Config.GetStringSlice("files.root_paths")
	if len(rootPaths) == 0 {
		rootPaths = []string{"/"} // 默认允许访问整个文件系统
	}

	// 设置全局允许的根目录
	common.SetAllowedRoots(rootPaths)

	// 从配置获取缩略图缓存目录
	thumbnailCacheDir := ctx.Config.GetString("files.thumbnail_cache_dir")
	if thumbnailCacheDir == "" {
		thumbnailCacheDir = "/var/cache/rde/thumbnails"
	}

	// 创建服务
	m.service = NewService(ctx.Logger, rootPaths)

	// 创建缩略图服务
	m.thumbnails = NewThumbnailService(ctx.Logger, thumbnailCacheDir)

	// 创建处理器
	m.handler = NewHandler(m.service, m.thumbnails, ctx.DB)

	ctx.Logger.Info("files module initialized",
		zap.Strings("root_paths", rootPaths),
		zap.String("thumbnail_cache_dir", thumbnailCacheDir))

	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	m.ctx.Logger.Info("files module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	m.ctx.Logger.Info("files module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.RegisterRoutes(group)
}

// GetService 获取服务实例（供其他模块调用）
func (m *Module) GetService() *Service {
	return m.service
}

// GetThumbnailService 获取缩略图服务（供其他模块调用）
func (m *Module) GetThumbnailService() *ThumbnailService {
	return m.thumbnails
}

// New 创建文件模块
func New() *Module {
	return &Module{}
}
