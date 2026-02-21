// Package module 提供模块化架构的核心定义和加载器
package module

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Module 定义了所有功能模块必须实现的接口
type Module interface {
	// ID 返回模块的唯一标识符，如 "files", "users"
	ID() string

	// Name 返回模块的显示名称
	Name() string

	// Version 返回模块版本
	Version() string

	// Dependencies 返回此模块依赖的其他模块 ID 列表
	// 模块加载器会确保依赖的模块先初始化
	Dependencies() []string

	// Init 初始化模块，传入核心上下文
	// 在此阶段应该：创建数据库表、初始化内部状态
	Init(ctx *Context) error

	// Start 启动模块
	// 在所有模块 Init 完成后调用
	// 在此阶段可以：启动后台任务、订阅事件
	Start() error

	// Stop 停止模块
	// 在应用关闭时调用
	// 在此阶段应该：清理资源、停止后台任务
	Stop() error

	// RegisterRoutes 注册模块的 HTTP 路由
	// router 是带有 /api/v1 前缀的路由组
	RegisterRoutes(router *gin.RouterGroup)
}

// Context 是传递给模块的核心上下文
// 包含模块运行所需的所有核心服务
type Context struct {
	// DB 是共享的数据库连接
	DB *gorm.DB

	// Config 提供配置读取能力
	Config ConfigProvider

	// EventBus 用于模块间异步通信
	EventBus EventBus

	// Logger 是模块专用的日志记录器
	Logger *zap.Logger

	// GetModule 获取其他已加载的模块
	// 用于模块间同步调用
	GetModule func(id string) Module

	// Extra 扩展数据（用于传递 TokenManager 等核心组件）
	Extra map[string]interface{}
}

// ConfigProvider 定义配置读取接口
type ConfigProvider interface {
	Get(key string) interface{}
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	GetStringSlice(key string) []string
}

// EventBus 定义事件总线接口
type EventBus interface {
	// Publish 发布事件
	Publish(eventType string, data interface{})

	// Subscribe 订阅事件
	Subscribe(eventType string, handler EventHandler)

	// Unsubscribe 取消订阅
	Unsubscribe(eventType string, handler EventHandler)
}

// EventHandler 事件处理函数类型
type EventHandler func(event Event)

// Event 事件结构
type Event struct {
	Type      string      // 事件类型，如 "user.created", "file.uploaded"
	Source    string      // 来源模块 ID
	Data      interface{} // 事件数据
	Timestamp int64       // Unix 时间戳
}

// ModuleInfo 模块基本信息（用于不需要完整 Module 接口的场景）
type ModuleInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Dependencies []string `json:"dependencies"`
	Status       string   `json:"status"` // "stopped", "starting", "running", "error"
}

// OptionalModule 可选模块扩展接口
// 可选模块应该实现此接口以提供额外的元信息
type OptionalModule interface {
	Module

	// IsOptional 返回模块是否可被用户禁用
	// 核心模块返回 false，可选模块返回 true
	IsOptional() bool

	// Description 返回模块描述
	Description() string

	// DefaultConfig 返回模块的默认配置定义
	// 用于前端渲染配置表单
	DefaultConfig() []ConfigField
}

// ConfigField 配置字段定义
type ConfigField struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Type        string      `json:"type"` // string/number/bool/select
	Default     interface{} `json:"default"`
	Options     []string    `json:"options,omitempty"` // select 选项
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
}

// IsModuleOptional 检查模块是否为可选模块
func IsModuleOptional(m Module) bool {
	if opt, ok := m.(OptionalModule); ok {
		return opt.IsOptional()
	}
	return false
}

// GetModuleDescription 获取模块描述
func GetModuleDescription(m Module) string {
	if opt, ok := m.(OptionalModule); ok {
		return opt.Description()
	}
	return ""
}

// GetModuleDefaultConfig 获取模块默认配置
func GetModuleDefaultConfig(m Module) []ConfigField {
	if opt, ok := m.(OptionalModule); ok {
		return opt.DefaultConfig()
	}
	return nil
}
