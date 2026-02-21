package sync

import "time"

// SyncStatus 同步服务状态
type SyncStatus struct {
	Running     bool   `json:"running"`
	StoragePath string `json:"storage_path"`
	TotalFiles  int    `json:"total_files"`
	TotalSize   int64  `json:"total_size"`
	Uploading   int    `json:"uploading"`
}

// SyncFile 已同步的文件
type SyncFile struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mime_type"`
	SHA256    string    `json:"sha256"`
	Path      string    `json:"path"`
	Status    string    `json:"status"` // uploading, completed, failed
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    string    `json:"user_id,omitempty"`
}

// UploadSession 上传会话 (断点续传)
type UploadSession struct {
	ID        string    `json:"id"`
	FileID    string    `json:"file_id,omitempty"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Offset    int64     `json:"offset"`
	Progress  float64   `json:"progress"`
	Metadata  string    `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
}

// StoreStats 存储统计
type StoreStats struct {
	TotalFiles int   `json:"total_files"`
	TotalSize  int64 `json:"total_size"`
	Uploading  int   `json:"uploading"`
}

// ListFilesRequest 文件列表请求
type ListFilesRequest struct {
	Path   string `form:"path"`
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

// ListFilesResponse 文件列表响应
type ListFilesResponse struct {
	Files []SyncFile `json:"files"`
	Total int        `json:"total"`
}
