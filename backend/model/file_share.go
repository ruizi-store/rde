package model

import (
	"time"
)

// FileShare 文件分享模型
type FileShare struct {
	ID           string     `json:"id" gorm:"primaryKey;size:36"`
	UserID       string     `json:"user_id" gorm:"index;size:36"`
	FilePath     string     `json:"file_path" gorm:"size:1024"`       // 原始文件路径
	FileName     string     `json:"file_name" gorm:"size:255"`        // 文件名
	IsDirectory  bool       `json:"is_directory"`                     // 是否为目录
	ShareCode    string     `json:"share_code" gorm:"uniqueIndex;size:16"` // 短链接码
	Password     string     `json:"password,omitempty" gorm:"size:64"`     // 访问密码（加密存储）
	ExpireAt     *time.Time `json:"expire_at"`                        // 过期时间，nil=永久
	MaxDownloads int        `json:"max_downloads"`                    // 最大下载次数，0=不限
	Downloads    int        `json:"downloads"`                        // 已下载次数
	ViewCount    int        `json:"view_count"`                       // 访问次数
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (FileShare) TableName() string {
	return "file_shares"
}

// CreateShareRequest 创建分享请求
type CreateShareRequest struct {
	FilePath     string `json:"file_path" binding:"required"`     // 文件路径
	Password     string `json:"password,omitempty"`               // 访问密码（可选）
	ExpireDays   int    `json:"expire_days"`                      // 过期天数，0=永久
	MaxDownloads int    `json:"max_downloads"`                    // 最大下载次数，0=不限
}

// ShareResponse 分享响应
type ShareResponse struct {
	ID           string     `json:"id"`
	ShareCode    string     `json:"share_code"`
	ShareURL     string     `json:"share_url"`
	FileName     string     `json:"file_name"`
	IsDirectory  bool       `json:"is_directory"`
	HasPassword  bool       `json:"has_password"`
	ExpireAt     *time.Time `json:"expire_at"`
	MaxDownloads int        `json:"max_downloads"`
	Downloads    int        `json:"downloads"`
	ViewCount    int        `json:"view_count"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ShareInfo 分享信息（公开访问）
type ShareInfo struct {
	ShareCode    string     `json:"share_code"`
	FileName     string     `json:"file_name"`
	IsDirectory  bool       `json:"is_directory"`
	FileSize     int64      `json:"file_size,omitempty"`
	HasPassword  bool       `json:"has_password"`
	ExpireAt     *time.Time `json:"expire_at,omitempty"`
	IsExpired    bool       `json:"is_expired"`
	CanDownload  bool       `json:"can_download"` // 是否还可下载（未超过次数限制）
}

// VerifyPasswordRequest 验证密码请求
type VerifyPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// ShareListItem 分享列表项
type ShareListItem struct {
	ID           string     `json:"id"`
	ShareCode    string     `json:"share_code"`
	ShareURL     string     `json:"share_url"`
	FilePath     string     `json:"file_path"`
	FileName     string     `json:"file_name"`
	IsDirectory  bool       `json:"is_directory"`
	HasPassword  bool       `json:"has_password"`
	ExpireAt     *time.Time `json:"expire_at"`
	IsExpired    bool       `json:"is_expired"`
	MaxDownloads int        `json:"max_downloads"`
	Downloads    int        `json:"downloads"`
	ViewCount    int        `json:"view_count"`
	CreatedAt    time.Time  `json:"created_at"`
}

// DirectoryContent 目录内容（用于分享目录时列出内容）
type DirectoryContent struct {
	Name       string    `json:"name"`
	IsDir      bool      `json:"is_dir"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}
