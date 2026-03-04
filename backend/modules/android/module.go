// Package android Android 设备管理模块
package android

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/module"
)

const (
	ModuleID      = "android"
	ModuleName    = "Android Device Manager"
	ModuleVersion = "1.0.0"
)

// Module Android 模块（实现 module.Module 接口）
type Module struct {
	ctx           *module.Context
	service       *Service
	installWizard *InstallWizard
	handler       *Handler
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
	return []string{"users", "docker"}
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.ctx = ctx
	logger := ctx.Logger
	m.service = NewService(logger)

	// 获取基础目录
	baseDir := ctx.Config.GetString("base_dir")
	if baseDir == "" {
		baseDir = "/opt/rde"
	}

	// Android Docker 镜像地址
	androidImage := ctx.Config.GetString("android.docker_image")
	if androidImage == "" {
		androidImage = "redroid/redroid:16.0.0-latest"
	}

	// binder-modules 路径改为 thirdparty/android/binder-modules
	installCfg := &InstallConfig{
		DockerImage:      androidImage,
		ContainerName:    "ruizios-android",
		BinderModulePath: filepath.Join(baseDir, "thirdparty", "android", "binder-modules", "binder"),
		ADBPort:          5555,
		DataVolume:       "ruizios-android-data",
	}
	m.installWizard = NewInstallWizard(installCfg)

	m.handler = NewHandler(m.service, m.installWizard, logger)

	ctx.Logger.Info("android module initialized")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	return nil
}

// Stop 停止模块
func (m *Module) Stop() error {
	if m.service != nil {
		m.service.Close()
	}
	return nil
}

// RegisterRoutes 注册路由
func (m *Module) RegisterRoutes(router *gin.RouterGroup) {
	m.handler.RegisterRoutes(router)
}
