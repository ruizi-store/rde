// Package docker 提供 Docker 容器管理和应用商店模块
package docker

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

const (
	ModuleID      = "docker"
	ModuleName    = "Docker App Store"
	ModuleVersion = "1.0.0"
)

// Module Docker应用模块
type Module struct {
	ctx              *module.Context
	service          *Service
	handler          *Handler
	catalog          *CatalogService
	storeHandler     *StoreHandler
	installed        *InstalledService
	installedHandler *InstalledHandler
	portHandler      *PortHandler
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
	return nil // Docker 模块无依赖
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx

	// 创建 Docker 服务
	service, err := NewService(ctx.Logger)
	if err != nil {
		ctx.Logger.Warn("Docker service init failed, running in degraded mode", zap.Error(err))
	}
	m.service = service

	// 创建容器管理处理器
	m.handler = NewHandler(m.service, ctx.Logger)

	// 加载应用商店目录
	m.initCatalog(ctx)

	// 初始化已安装应用服务（依赖 catalog）
	m.initInstalled(ctx)

	// 端口检测
	m.portHandler = NewPortHandler()

	ctx.Logger.Info("docker module initialized")
	return nil
}

// initCatalog 初始化应用商店目录
func (m *Module) initCatalog(ctx *module.Context) {
	// 查找 docker-apps.yaml：优先数据目录，其次前端 static 目录
	dataDir := ctx.Config.GetString("data_dir")
	candidates := []string{
		filepath.Join(dataDir, "docker-apps.yaml"),
		filepath.Join(dataDir, "www", "docker-apps.yaml"),
	}

	// 开发模式下也检查常见路径
	devPaths := []string{
		"frontend/static/docker-apps.yaml",
		"../frontend/static/docker-apps.yaml",
	}
	candidates = append(candidates, devPaths...)

	var yamlPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			yamlPath = p
			break
		}
	}

	if yamlPath == "" {
		ctx.Logger.Warn("docker-apps.yaml not found, app store disabled")
		return
	}

	catalog, err := NewCatalogService(yamlPath, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Failed to load app catalog", zap.Error(err), zap.String("path", yamlPath))
		return
	}

	m.catalog = catalog
	m.storeHandler = NewStoreHandler(catalog, ctx.Logger)

	// 查找图标目录
	iconCandidates := []string{
		filepath.Join(dataDir, "docker-icons"),
		filepath.Join(dataDir, "www", "docker-icons"),
		"frontend/static/docker-icons",
		"../frontend/static/docker-icons",
	}
	for _, p := range iconCandidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			m.storeHandler.SetIconsDir(p)
			ctx.Logger.Info("Docker icons directory found", zap.String("path", p))
			break
		}
	}

	ctx.Logger.Info("App store catalog loaded",
		zap.String("path", yamlPath),
		zap.Int("apps", catalog.AppCount()),
	)
}

// initInstalled 初始化已安装应用服务
func (m *Module) initInstalled(ctx *module.Context) {
	if m.catalog == nil {
		ctx.Logger.Warn("Catalog not loaded, installed apps service disabled")
		return
	}

	dataDir := ctx.Config.GetString("data_dir")
	installed, err := NewInstalledService(dataDir, m.catalog, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Failed to init installed apps service", zap.Error(err))
		return
	}

	m.installed = installed
	m.installedHandler = NewInstalledHandler(installed, ctx.Logger)
}

// Start 启动模块
func (m *Module) Start() error {
	m.ctx.Logger.Info("docker module started")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	if m.service != nil {
		m.service.Close()
	}
	m.ctx.Logger.Info("docker module stopped")
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	docker := group.Group("/docker")
	m.handler.RegisterRoutes(docker)

	// 应用商店路由
	if m.storeHandler != nil {
		store := docker.Group("/store")
		m.storeHandler.RegisterRoutes(store)
	}

	// 已安装应用路由
	if m.installedHandler != nil {
		apps := docker.Group("/apps")
		m.installedHandler.RegisterRoutes(apps)
	}

	// 端口检测路由
	if m.portHandler != nil {
		ports := docker.Group("/ports")
		m.portHandler.RegisterRoutes(ports)
	}
}

// GetService 获取服务实例（供其他模块调用）
func (m *Module) GetService() *Service {
	return m.service
}

// GetCatalog 获取目录服务实例（供其他模块调用）
func (m *Module) GetCatalog() *CatalogService {
	return m.catalog
}
