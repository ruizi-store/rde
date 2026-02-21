// Package event 预定义的事件类型常量
package event

// 用户相关事件
const (
	UserCreated  = "user.created"
	UserUpdated  = "user.updated"
	UserDeleted  = "user.deleted"
	UserLoggedIn = "user.logged_in"
)

// 文件相关事件
const (
	FileUploaded  = "file.uploaded"
	FileDeleted   = "file.deleted"
	FileMoved     = "file.moved"
	FileRenamed   = "file.renamed"
	FileShared    = "file.shared"
	FolderCreated = "folder.created"
)

// 应用商店相关事件
const (
	AppInstalled   = "app.installed"
	AppUninstalled = "app.uninstalled"
	AppStarted     = "app.started"
	AppStopped     = "app.stopped"
)

// 系统相关事件
const (
	SystemStartup  = "system.startup"
	SystemShutdown = "system.shutdown"
)

// 通知相关事件
const (
	NotificationCreated = "notification.created"
)
