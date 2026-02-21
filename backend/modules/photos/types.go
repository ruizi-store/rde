// Package photos 照片管理模块
package photos

import "time"

// ============ 请求类型 ============

// CreateLibraryRequest 创建图库请求
type CreateLibraryRequest struct {
	Name        string `json:"name" binding:"required"`
	Path        string `json:"path" binding:"required"`
	ScanEnabled bool   `json:"scan_enabled"`
}

// UpdateLibraryRequest 更新图库请求
type UpdateLibraryRequest struct {
	Name        *string `json:"name"`
	ScanEnabled *bool   `json:"scan_enabled"`
}

// ListPhotosRequest 照片列表请求
type ListPhotosRequest struct {
	LibraryID string `form:"library_id"`
	AlbumID   string `form:"album_id"`
	Type      string `form:"type"`       // photo, video, all
	Favorite  *bool  `form:"favorite"`   // 只显示收藏
	Archived  *bool  `form:"archived"`   // 只显示归档
	StartDate string `form:"start_date"` // YYYY-MM-DD
	EndDate   string `form:"end_date"`   // YYYY-MM-DD
	Offset    int    `form:"offset"`
	Limit     int    `form:"limit"`
	Sort      string `form:"sort"` // date_asc, date_desc, name_asc, name_desc
}

// TimelineRequest 时间线请求
type TimelineRequest struct {
	LibraryID string `form:"library_id"`
	GroupBy   string `form:"group_by"` // day, month, year
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query       string   `form:"q" binding:"required"`
	LibraryID   string   `form:"library_id"`
	Type        string   `form:"type"`
	CameraMake  string   `form:"camera_make"`
	CameraModel string   `form:"camera_model"`
	StartDate   string   `form:"start_date"`
	EndDate     string   `form:"end_date"`
	HasLocation *bool    `form:"has_location"`
	Tags        []string `form:"tags"`
	Offset      int      `form:"offset"`
	Limit       int      `form:"limit"`
}

// CreateAlbumRequest 创建相册请求
type CreateAlbumRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	PhotoIDs    []string `json:"photo_ids"`
}

// UpdateAlbumRequest 更新相册请求
type UpdateAlbumRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	CoverID     *string `json:"cover_id"`
	SortOrder   *string `json:"sort_order"`
}

// AddPhotosToAlbumRequest 添加照片到相册
type AddPhotosToAlbumRequest struct {
	PhotoIDs []string `json:"photo_ids" binding:"required"`
}

// UpdatePhotoRequest 更新照片请求
type UpdatePhotoRequest struct {
	IsFavorite *bool      `json:"is_favorite"`
	IsArchived *bool      `json:"is_archived"`
	TakenAt    *time.Time `json:"taken_at"`
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	PhotoIDs []string `json:"photo_ids" binding:"required"`
	Force    bool     `json:"force"` // 永久删除
}

// ============ 响应类型 ============

// LibraryResponse 图库响应
type LibraryResponse struct {
	Library
	Scanning bool `json:"scanning"`
}

// PhotoResponse 照片响应
type PhotoResponse struct {
	Photo
	ThumbnailURL string `json:"thumbnail_url"`
	PreviewURL   string `json:"preview_url"`
	OriginalURL  string `json:"original_url"`
}

// ListPhotosResponse 照片列表响应
type ListPhotosResponse struct {
	Photos []PhotoResponse `json:"photos"`
	Total  int64           `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

// TimelineGroup 时间线分组
type TimelineGroup struct {
	Date   string          `json:"date"`   // YYYY-MM-DD, YYYY-MM, YYYY
	Count  int             `json:"count"`  // 照片数量
	Photos []PhotoResponse `json:"photos"` // 照片列表
}

// TimelineResponse 时间线响应
type TimelineResponse struct {
	Groups []TimelineGroup `json:"groups"`
	Total  int64           `json:"total"`
}

// CalendarDay 日历日期数据
type CalendarDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// CalendarResponse 日历响应
type CalendarResponse struct {
	Days []CalendarDay `json:"days"`
}

// AlbumResponse 相册响应
type AlbumResponse struct {
	Album
	CoverURL string `json:"cover_url,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	TotalPhotos   int64 `json:"total_photos"`
	TotalVideos   int64 `json:"total_videos"`
	TotalSize     int64 `json:"total_size"`
	TotalAlbums   int64 `json:"total_albums"`
	FavoriteCount int64 `json:"favorite_count"`
	ArchivedCount int64 `json:"archived_count"`
	TrashCount    int64 `json:"trash_count"`
}

// ScanProgress 扫描进度
type ScanProgress struct {
	LibraryID    string `json:"library_id"`
	Status       string `json:"status"` // idle, scanning, indexing, completed
	TotalFiles   int    `json:"total_files"`
	ScannedFiles int    `json:"scanned_files"`
	IndexedFiles int    `json:"indexed_files"`
	FailedFiles  int    `json:"failed_files"`
	StartedAt    string `json:"started_at,omitempty"`
	CompletedAt  string `json:"completed_at,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ============ 配置类型 ============

// Config 模块配置
type Config struct {
	ThumbnailCacheDir string `mapstructure:"thumbnail_cache_dir"`
	OriginalsCacheDir string `mapstructure:"originals_cache_dir"`
	ScanInterval      int    `mapstructure:"scan_interval"`   // 扫描间隔（分钟）
	MaxScanWorkers    int    `mapstructure:"max_scan_workers"` // 最大扫描并发数
	EnableAI          bool   `mapstructure:"enable_ai"`
	IndexerURL        string `mapstructure:"indexer_url"` // AI 索引服务地址
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		ThumbnailCacheDir: "/var/cache/rde/photos/thumbnails",
		OriginalsCacheDir: "/var/cache/rde/photos/originals",
		ScanInterval:      60,
		MaxScanWorkers:    4,
		EnableAI:          false,
		IndexerURL:        "http://localhost:8081",
	}
}
