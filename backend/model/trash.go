package model

import (
	"time"
)

// TrashItem 回收站项目模型
type TrashItem struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	UserID       string    `json:"user_id" gorm:"index;size:36"`
	OriginalPath string    `json:"original_path" gorm:"size:1024"`   // 原始路径
	TrashPath    string    `json:"trash_path" gorm:"size:1024"`      // 回收站中的路径
	FileName     string    `json:"file_name" gorm:"size:255"`        // 文件名
	IsDirectory  bool      `json:"is_directory"`                     // 是否为目录
	Size         int64     `json:"size"`                             // 文件大小
	DeletedAt    time.Time `json:"deleted_at" gorm:"index"`          // 删除时间
	ExpireAt     time.Time `json:"expire_at" gorm:"index"`           // 自动清理时间
}

// TableName 表名
func (TrashItem) TableName() string {
	return "trash_items"
}

// TrashStats 回收站统计
type TrashStats struct {
	TotalCount int64 `json:"total_count"` // 总数量
	TotalSize  int64 `json:"total_size"`  // 总大小
	FileCount  int64 `json:"file_count"`  // 文件数量
	DirCount   int64 `json:"dir_count"`   // 目录数量
}

// TrashListItem 回收站列表项（API 响应）
type TrashListItem struct {
	ID           string    `json:"id"`
	FileName     string    `json:"file_name"`
	OriginalPath string    `json:"original_path"`
	IsDirectory  bool      `json:"is_directory"`
	Size         int64     `json:"size"`
	DeletedAt    time.Time `json:"deleted_at"`
	ExpireAt     time.Time `json:"expire_at"`
	DaysLeft     int       `json:"days_left"` // 剩余天数
}

// MoveToTrashRequest 移动到回收站请求
type MoveToTrashRequest struct {
	Paths []string `json:"paths" binding:"required"` // 要删除的路径列表
}

// TrashRestoreRequest 回收站还原请求
type TrashRestoreRequest struct {
	IDs []string `json:"ids" binding:"required"` // 要还原的项目 ID 列表
}

// DeletePermanentlyRequest 永久删除请求
type DeletePermanentlyRequest struct {
	IDs []string `json:"ids" binding:"required"` // 要永久删除的项目 ID 列表
}

// TrashConfig 回收站配置
type TrashConfig struct {
	RetentionDays int   `json:"retention_days"` // 保留天数，默认 30
	MaxSize       int64 `json:"max_size"`       // 最大容量（字节），0=不限
	Enabled       bool  `json:"enabled"`        // 是否启用回收站
}

// DefaultTrashConfig 默认回收站配置
var DefaultTrashConfig = TrashConfig{
	RetentionDays: 30,
	MaxSize:       0, // 不限制
	Enabled:       true,
}
