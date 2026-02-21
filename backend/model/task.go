package model

import (
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 排队中
	TaskStatusRunning   TaskStatus = "running"   // 运行中
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 失败
)

// TaskPhase 任务阶段
type TaskPhase string

const (
	TaskPhaseInit              TaskPhase = "init"               // 初始化
	TaskPhasePullingImage      TaskPhase = "pulling_image"      // 拉取镜像
	TaskPhaseCreatingNetwork   TaskPhase = "creating_network"   // 创建网络
	TaskPhaseCreatingContainer TaskPhase = "creating_container" // 创建容器
	TaskPhaseStartingContainer TaskPhase = "starting_container" // 启动容器
	TaskPhaseCompleted         TaskPhase = "completed"          // 完成
	TaskPhaseFailed            TaskPhase = "failed"             // 失败
)

// InstallTask 安装任务
type InstallTask struct {
	ID             string     `json:"id" gorm:"primaryKey"`            // 任务ID (UUID)
	AppID          string     `json:"app_id" gorm:"index;not null"`    // 应用ID
	AppName        string     `json:"app_name"`                        // 应用名称
	AppIcon        string     `json:"app_icon"`                        // 应用图标
	Status         TaskStatus `json:"status" gorm:"default:'pending'"` // 任务状态
	Phase          TaskPhase  `json:"phase" gorm:"default:'init'"`     // 当前阶段
	Progress       int        `json:"progress" gorm:"default:0"`       // 进度百分比 0-100
	Message        string     `json:"message"`                         // 当前操作消息
	ErrorMessage   string     `json:"error_message"`                   // 错误信息
	Logs           string     `json:"logs" gorm:"type:text"`           // 安装日志 (JSON数组)
	Config         string     `json:"config" gorm:"type:text"`         // 安装配置 (JSON)
	InstalledAppID uint       `json:"installed_app_id"`                // 安装完成后的应用ID
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (InstallTask) TableName() string {
	return "install_tasks"
}

// InstallTaskLog 安装日志条目
type InstallTaskLog struct {
	Time    time.Time `json:"time"`
	Phase   TaskPhase `json:"phase"`
	Message string    `json:"message"`
}

// InstallTaskResponse 安装任务响应
type InstallTaskResponse struct {
	TaskID string `json:"task_id"`
}

// InstallTasksResponse 任务列表响应
type InstallTasksResponse struct {
	Tasks []InstallTask `json:"tasks"`
}
