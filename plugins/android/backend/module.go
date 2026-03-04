// Package android Android 设备管理模块
package android

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde-plugin-common/go/sdk"
)

// Module Android 模块（实现 sdk.PluginModule 接口）
type Module struct {
	service       *Service
	installWizard *InstallWizard
	handler       *Handler
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "android" }

// Init 初始化模块
func (m *Module) Init(ctx *sdk.PluginContext) error {
	logger := ctx.Logger
	m.service = NewService(logger)

	baseDir := ctx.BaseDir
	if baseDir == "" {
		baseDir = "/opt/rde"
	}

	// Android Docker 镜像地址（独立模块不依赖 i18n，使用默认值）
	androidImage := "redroid/redroid:16.0.0-latest"

	installCfg := &InstallConfig{
		DockerImage:      androidImage,
		ContainerName:    "ruizios-android",
		BinderModulePath: filepath.Join(baseDir, "plugins", "android", "binder-modules", "binder"),
		ADBPort:          5555,
		DataVolume:       "ruizios-android-data",
	}
	m.installWizard = NewInstallWizard(installCfg)

	m.handler = NewHandler(m.service, m.installWizard, logger)
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
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	m.handler.RegisterRoutes(group)
}
