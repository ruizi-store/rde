// Package notification 通知模块
package notification

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"github.com/ruizi-store/rde/backend/core/module"
	"go.uber.org/zap"
)

// Module 通知模块
type Module struct {
	service  *Service
	handler  *Handler
	eventBus module.EventBus
	logger   *zap.Logger
}

// New 创建模块实例
func New() *Module {
	return &Module{}
}

// ID 返回模块唯一标识
func (m *Module) ID() string {
	return "notification"
}

// Name 返回模块名称
func (m *Module) Name() string {
	return "Notification"
}

// Version 返回模块版本
func (m *Module) Version() string {
	return "2.0.0"
}

// Dependencies 返回依赖的模块
func (m *Module) Dependencies() []string {
	return []string{} // 通知模块无依赖
}

// Init 初始化模块
func (m *Module) Init(ctx *module.Context) error {
	m.service = NewService(ctx.DB, ctx.Logger)

	// 从 Extra 获取 TokenManager
	var tokenManager *auth.TokenManager
	if tm, ok := ctx.Extra["tokenManager"].(*auth.TokenManager); ok {
		tokenManager = tm
	}

	m.handler = NewHandler(m.service, tokenManager)
	m.eventBus = ctx.EventBus
	m.logger = ctx.Logger

	// 启动 WebSocket Hub
	go m.service.GetHub().run()

	// 迁移数据库
	if err := m.service.Migrate(); err != nil {
		return err
	}

	// 订阅各模块事件
	m.subscribeEvents()

	return nil
}

// Start 启动模块
func (m *Module) Start() error {
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

// GetService 获取服务实例
func (m *Module) GetService() *Service {
	return m.service
}

// SendNotification 发送通知（供其他模块调用）
func (m *Module) SendNotification(userID string, category Category, severity Severity, title, content, link, source string) error {
	_, err := m.service.SendNotification(nil, &SendNotificationRequest{
		UserID:   userID,
		Category: category,
		Severity: severity,
		Title:    title,
		Content:  content,
		Link:     link,
		Source:   source,
	})
	return err
}

// NewModule 创建通知模块（别名）
func NewModule() *Module {
	return New()
}

// subscribeEvents 订阅各模块事件
func (m *Module) subscribeEvents() {
	if m.eventBus == nil {
		return
	}

	// 订阅存储健康告警
	m.eventBus.Subscribe("storage.health.alert", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}

		level, _ := data["level"].(string)
		message, _ := data["message"].(string)
		diskPath, _ := data["disk_path"].(string)

		severity := SeverityWarning
		if level == "critical" {
			severity = SeverityCritical
		}

		m.SendNotification("", CategoryStorage, severity,
			"磁盘健康告警",
			message+" ("+diskPath+")",
			"/storage", "storage")
	})

	// 订阅磁盘格式化
	m.eventBus.Subscribe("storage.formatted", func(e module.Event) {
		data, ok := e.Data.(map[string]string)
		if !ok {
			return
		}
		m.SendNotification("", CategoryStorage, SeverityInfo,
			"磁盘格式化完成",
			"磁盘 "+data["path"]+" 已格式化为 "+data["filesystem"],
			"/storage", "storage")
	})

	// 订阅 RAID 创建
	m.eventBus.Subscribe("storage.raid.created", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		level, _ := data["level"].(string)
		m.SendNotification("", CategoryStorage, SeverityInfo,
			"RAID 阵列创建成功",
			"已创建 "+level+" 阵列: "+name,
			"/storage", "storage")
	})

	// 订阅 RAID 删除
	m.eventBus.Subscribe("storage.raid.deleted", func(e module.Event) {
		data, ok := e.Data.(map[string]string)
		if !ok {
			return
		}
		m.SendNotification("", CategoryStorage, SeverityWarning,
			"RAID 阵列已删除",
			"RAID 阵列 "+data["name"]+" 已被删除",
			"/storage", "storage")
	})

	// 订阅 LVM 卷组创建
	m.eventBus.Subscribe("storage.lvm.vg.created", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		m.SendNotification("", CategoryStorage, SeverityInfo,
			"存储卷组创建成功",
			"已创建卷组: "+name,
			"/storage", "storage")
	})

	// 订阅 LVM 逻辑卷创建
	m.eventBus.Subscribe("storage.lvm.lv.created", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		vgName, _ := data["vg_name"].(string)
		m.SendNotification("", CategoryStorage, SeverityInfo,
			"存储卷创建成功",
			"已在卷组 "+vgName+" 中创建逻辑卷: "+name,
			"/storage", "storage")
	})

	// 订阅快照创建
	m.eventBus.Subscribe("snapshot.created", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		source, _ := data["source"].(string)
		m.SendNotification("", CategoryBackup, SeverityInfo,
			"快照创建成功",
			"已为 "+source+" 创建快照: "+name,
			"/storage", "storage")
	})

	// 订阅快照恢复
	m.eventBus.Subscribe("snapshot.restored", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		m.SendNotification("", CategoryBackup, SeverityInfo,
			"快照恢复成功",
			"快照 "+name+" 已成功恢复",
			"/storage", "storage")
	})

	// 订阅数据迁移开始
	m.eventBus.Subscribe("migration.started", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		source, _ := data["source"].(string)
		target, _ := data["target"].(string)
		m.SendNotification("", CategoryStorage, SeverityInfo,
			"数据迁移已开始",
			"正在从 "+source+" 迁移数据到 "+target,
			"/storage", "storage")
	})

	// 订阅存储卷大小调整
	m.eventBus.Subscribe("volume.resized", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		m.SendNotification("", CategoryStorage, SeverityInfo,
			"存储卷调整大小成功",
			"存储卷 "+name+" 大小已调整",
			"/storage", "storage")
	})

	// 订阅磁盘热插拔事件
	m.eventBus.Subscribe("storage.disk.added", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		path, _ := data["path"].(string)
		m.SendNotification("", CategoryStorage, SeverityInfo,
			"检测到新磁盘",
			"已检测到新磁盘: "+path,
			"/storage", "storage")
	})

	m.eventBus.Subscribe("storage.disk.removed", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		path, _ := data["path"].(string)
		m.SendNotification("", CategoryStorage, SeverityWarning,
			"磁盘已移除",
			"磁盘已被移除: "+path,
			"/storage", "storage")
	})

	m.logger.Info("Notification module subscribed to storage events")

	// ========== 用户模块事件 ==========

	// 订阅登录成功事件
	m.eventBus.Subscribe("users.login.success", func(e module.Event) {
		data, ok := e.Data.(map[string]string)
		if !ok {
			return
		}
		userID := data["user_id"]
		username := data["username"]
		ip := data["ip"]
		m.SendNotification(userID, CategorySecurity, SeverityInfo,
			"登录提醒",
			"您的账户已登录，IP: "+ip,
			"/settings/security", "users")
		m.logger.Info("Login notification sent", zap.String("username", username))
	})

	// 订阅登录失败事件
	m.eventBus.Subscribe("users.login.failed", func(e module.Event) {
		data, ok := e.Data.(map[string]string)
		if !ok {
			return
		}
		username := data["username"]
		ip := data["ip"]
		reason := data["reason"]
		content := "用户 " + username + " 登录失败，IP: " + ip
		if reason == "user_disabled" {
			content = "被禁用的用户 " + username + " 尝试登录，IP: " + ip
		}
		// 广播给所有管理员
		m.SendNotification("", CategorySecurity, SeverityWarning,
			"登录失败尝试",
			content,
			"/settings/security", "users")
	})

	// 订阅密码修改事件
	m.eventBus.Subscribe("users.password.changed", func(e module.Event) {
		data, ok := e.Data.(map[string]string)
		if !ok {
			return
		}
		userID := data["user_id"]
		m.SendNotification(userID, CategorySecurity, SeverityInfo,
			"密码已修改",
			"您的账户密码已成功修改，如非本人操作请立即联系管理员",
			"/settings/account", "users")
	})

	// 订阅用户删除事件
	m.eventBus.Subscribe("users.deleted", func(e module.Event) {
		data, ok := e.Data.(map[string]string)
		if !ok {
			return
		}
		username := data["username"]
		m.SendNotification("", CategorySystem, SeverityInfo,
			"用户已删除",
			"用户 "+username+" 已被删除",
			"/settings/users", "users")
	})

	m.logger.Info("Notification module subscribed to users events")

	// ========== 套件模块事件 ==========

	// 订阅套件安装成功事件
	m.eventBus.Subscribe("packages.installed", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		version, _ := data["version"].(string)
		m.SendNotification("", CategoryApp, SeverityInfo,
			"套件安装成功",
			"套件 "+name+" ("+version+") 已成功安装",
			"/apps", "packages")
	})

	// 订阅套件安装失败事件
	m.eventBus.Subscribe("packages.install.failed", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		errMsg, _ := data["error"].(string)
		title := "套件安装失败"
		content := errMsg
		if name != "" {
			title = name + " 安装失败"
		}
		m.SendNotification("", CategoryApp, SeverityError,
			title,
			content,
			"/apps", "packages")
	})

	// 订阅套件卸载事件
	m.eventBus.Subscribe("packages.uninstalled", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		m.SendNotification("", CategoryApp, SeverityInfo,
			"套件已卸载",
			"套件 "+name+" 已成功卸载",
			"/apps", "packages")
	})

	// 订阅套件升级成功事件
	m.eventBus.Subscribe("packages.upgraded", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		version, _ := data["version"].(string)
		m.SendNotification("", CategoryUpdate, SeverityInfo,
			"套件已更新",
			"套件 "+name+" 已更新到版本 "+version,
			"/apps", "packages")
	})

	// 订阅套件升级失败事件
	m.eventBus.Subscribe("packages.upgrade.failed", func(e module.Event) {
		data, ok := e.Data.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := data["name"].(string)
		errMsg, _ := data["error"].(string)
		title := "套件更新失败"
		content := errMsg
		if name != "" {
			title = name + " 更新失败"
		}
		m.SendNotification("", CategoryUpdate, SeverityError,
			title,
			content,
			"/apps", "packages")
	})

	m.logger.Info("Notification module subscribed to packages events")

	// ========== 系统模块事件 ==========

	// 订阅系统重启事件
	m.eventBus.Subscribe("system.reboot", func(e module.Event) {
		m.SendNotification("", CategorySystem, SeverityWarning,
			"系统即将重启",
			"系统即将重启，请保存未完成的工作",
			"", "system")
	})

	// 订阅系统关机事件
	m.eventBus.Subscribe("system.shutdown", func(e module.Event) {
		m.SendNotification("", CategorySystem, SeverityWarning,
			"系统即将关机",
			"系统即将关机，请保存未完成的工作",
			"", "system")
	})

	// 订阅 SSH 启用事件
	m.eventBus.Subscribe("system.ssh.enabled", func(e module.Event) {
		m.SendNotification("", CategorySecurity, SeverityInfo,
			"SSH 服务已启用",
			"SSH 远程访问服务已启用",
			"/settings/remote-access", "system")
	})

	// 订阅 SSH 禁用事件
	m.eventBus.Subscribe("system.ssh.disabled", func(e module.Event) {
		m.SendNotification("", CategorySecurity, SeverityInfo,
			"SSH 服务已禁用",
			"SSH 远程访问服务已禁用",
			"/settings/remote-access", "system")
	})

	m.logger.Info("Notification module subscribed to system events")

	// ========== 特权操作模块事件 ==========

	// 订阅特权授权请求事件（直接广播到 WebSocket，不存储为通知）
	m.eventBus.Subscribe("privilege.authorization_request", func(e module.Event) {
		// 直接通过 WebSocket 广播给所有客户端
		m.service.GetHub().Broadcast(&WebSocketMessage{
			Type: "privilege.authorization_request",
			Data: e.Data,
		})
		m.logger.Debug("Privilege authorization request broadcasted via WebSocket")
	})

	m.logger.Info("Notification module subscribed to privilege events")
}
