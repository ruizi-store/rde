// Package files 提供文件管理模块
package files

import (
	"time"
)

// FileInfo 文件/目录信息
type FileInfo struct {
	Name       string                 `json:"name"`
	Path       string                 `json:"path"`
	Size       int64                  `json:"size"`
	IsDir      bool                   `json:"is_dir"`
	IsSymlink  bool                   `json:"is_symlink,omitempty"`
	LinkTarget string                 `json:"link_target,omitempty"`
	ModTime    time.Time              `json:"modified"`
	Mode       string                 `json:"mode,omitempty"`
	Owner      string                 `json:"owner,omitempty"`
	Group      string                 `json:"group,omitempty"`
	MimeType   string                 `json:"mime_type,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// ListRequest 目录列表请求
type ListRequest struct {
	Path       string `json:"path" form:"path"`
	Index      int    `json:"index" form:"index"`             // 页码，从1开始
	Size       int    `json:"size" form:"size"`               // 每页数量
	ShowHidden bool   `json:"show_hidden" form:"show_hidden"` // 是否显示隐藏文件
}

// ListResponse 目录列表响应
type ListResponse struct {
	Content      []FileInfo `json:"content"`
	Total        int64      `json:"total"`
	Index        int        `json:"index"`
	Size         int        `json:"size"`
	ResolvedPath string     `json:"resolved_path,omitempty"` // 符号链接解析后的真实路径
}

// FileOperation 文件操作（复制/移动）
type FileOperation struct {
	Type          string     `json:"type" binding:"required"` // move, copy
	Items         []FileItem `json:"items" binding:"required"`
	Destination   string     `json:"destination" binding:"required"`
	ConflictStyle string     `json:"conflict_style"` // skip, overwrite
	TotalSize     int64      `json:"total_size"`
	ProcessedSize int64      `json:"processed_size"`
	Finished      bool       `json:"finished"`
	Username      string     `json:"-"` // 执行操作的用户名，不从 JSON 读取
}

// FileItem 操作项
type FileItem struct {
	Path          string `json:"path" binding:"required"`
	Size          int64  `json:"size"`
	ProcessedSize int64  `json:"processed_size"`
	Finished      bool   `json:"finished"`
}

// OperationStatus 操作状态
type OperationStatus struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	TotalSize     int64  `json:"total_size"`
	ProcessedSize int64  `json:"processed_size"`
	Progress      int    `json:"progress"` // 0-100
	Finished      bool   `json:"finished"`
}

// CreateRequest 创建文件/目录请求
type CreateRequest struct {
	Path    string `json:"path" binding:"required"`
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Content string `json:"content,omitempty"`
}

// RenameRequest 重命名请求
type RenameRequest struct {
	OldPath string `json:"old_path" binding:"required"`
	NewPath string `json:"new_path" binding:"required"`
}

// UpdateContentRequest 更新文件内容请求
type UpdateContentRequest struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// UploadChunkInfo 分片上传信息
type UploadChunkInfo struct {
	FileName     string `json:"filename" form:"filename"`
	RelativePath string `json:"relative_path" form:"relativePath"`
	ChunkNumber  int    `json:"chunk_number" form:"chunkNumber"`
	TotalChunks  int    `json:"total_chunks" form:"totalChunks"`
	Path         string `json:"path" form:"path"` // 目标目录
}

// DownloadRequest 下载请求
type DownloadRequest struct {
	Files  []string `json:"files" form:"files"`   // 文件列表
	Format string   `json:"format" form:"format"` // 压缩格式: zip, tar, targz
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Path       string `json:"path" form:"path"`
	Keyword    string `json:"keyword" form:"keyword" binding:"required"`
	Recursive  bool   `json:"recursive" form:"recursive"`
	FileType   string `json:"file_type" form:"file_type"` // all, file, dir
	MaxResults int    `json:"max_results" form:"max_results"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Files []FileInfo `json:"files"`
	Total int        `json:"total"`
}

// DiskUsage 磁盘使用情况
type DiskUsage struct {
	Path    string  `json:"path"`
	Total   int64   `json:"total"`
	Used    int64   `json:"used"`
	Free    int64   `json:"free"`
	UsedPct float64 `json:"used_percent"`
}

// FileStats 文件统计
type FileStats struct {
	TotalFiles int64 `json:"total_files"`
	TotalDirs  int64 `json:"total_dirs"`
	TotalSize  int64 `json:"total_size"`
}
