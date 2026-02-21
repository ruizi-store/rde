package model

import "time"

// ThumbnailSize 缩略图尺寸
type ThumbnailSize int

const (
	ThumbnailSmall  ThumbnailSize = 128  // 小缩略图
	ThumbnailMedium ThumbnailSize = 256  // 中等缩略图
	ThumbnailLarge  ThumbnailSize = 512  // 大缩略图
	ThumbnailXLarge ThumbnailSize = 1024 // 特大缩略图
)

// ThumbnailCache 缩略图缓存记录
type ThumbnailCache struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	FilePath      string    `json:"file_path" gorm:"index"`
	FileHash      string    `json:"file_hash" gorm:"index"` // 原文件 MD5 用于检测变更
	ThumbnailPath string    `json:"thumbnail_path"`
	Size          int       `json:"size"` // 缩略图尺寸
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	Format        string    `json:"format"` // jpeg, png, webp
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ThumbnailCache) TableName() string {
	return "thumbnail_cache"
}

// ExifData EXIF 元数据
type ExifData struct {
	Make         string     `json:"make,omitempty"`          // 相机品牌
	Model        string     `json:"model,omitempty"`         // 相机型号
	DateTime     *time.Time `json:"date_time,omitempty"`     // 拍摄时间
	Width        int        `json:"width,omitempty"`         // 原始宽度
	Height       int        `json:"height,omitempty"`        // 原始高度
	Orientation  int        `json:"orientation,omitempty"`   // 方向
	FNumber      float64    `json:"f_number,omitempty"`      // 光圈值
	ExposureTime string     `json:"exposure_time,omitempty"` // 曝光时间
	ISOSpeed     int        `json:"iso_speed,omitempty"`     // ISO
	FocalLength  float64    `json:"focal_length,omitempty"`  // 焦距
	GPSLatitude  float64    `json:"gps_latitude,omitempty"`  // GPS 纬度
	GPSLongitude float64    `json:"gps_longitude,omitempty"` // GPS 经度
	Software     string     `json:"software,omitempty"`      // 软件
}

// VideoMetadata 视频元数据
type VideoMetadata struct {
	Duration    float64 `json:"duration"`     // 时长（秒）
	Width       int     `json:"width"`        // 宽度
	Height      int     `json:"height"`       // 高度
	Codec       string  `json:"codec"`        // 编解码器
	Bitrate     int64   `json:"bitrate"`      // 比特率
	FrameRate   float64 `json:"frame_rate"`   // 帧率
	AudioCodec  string  `json:"audio_codec"`  // 音频编解码器
	AudioRate   int     `json:"audio_rate"`   // 音频采样率
	HasSubtitle bool    `json:"has_subtitle"` // 是否有字幕
}

// FilePreviewInfo 文件预览信息
type FilePreviewInfo struct {
	Type         string         `json:"type"`                    // image, video, audio, document, code, text, unknown
	MimeType     string         `json:"mime_type"`               // MIME 类型
	ThumbnailURL string         `json:"thumbnail_url,omitempty"` // 缩略图 URL
	PreviewURL   string         `json:"preview_url,omitempty"`   // 预览 URL
	DownloadURL  string         `json:"download_url,omitempty"`  // 下载 URL
	Exif         *ExifData      `json:"exif,omitempty"`          // EXIF 数据（图片）
	VideoMeta    *VideoMetadata `json:"video_meta,omitempty"`    // 视频元数据
	CanPreview   bool           `json:"can_preview"`             // 是否支持预览
	CanEdit      bool           `json:"can_edit"`                // 是否支持编辑
}

// PreviewRequest 预览请求
type PreviewRequest struct {
	Path string `json:"path" query:"path"`
	Size int    `json:"size" query:"size"` // 缩略图尺寸
}

// GalleryItem 画廊项目
type GalleryItem struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	ThumbnailURL string    `json:"thumbnail_url"`
	PreviewURL   string    `json:"preview_url"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Exif         *ExifData `json:"exif,omitempty"`
}

// GalleryResponse 画廊响应
type GalleryResponse struct {
	Items      []GalleryItem `json:"items"`
	Total      int           `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

// SupportedImageFormats 支持的图片格式
var SupportedImageFormats = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".svg":  true,
	".bmp":  true,
	".ico":  true,
	".tiff": true,
	".heic": true,
	".heif": true,
}

// SupportedVideoFormats 支持的视频格式
var SupportedVideoFormats = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".3gp":  true,
}

// SupportedAudioFormats 支持的音频格式
var SupportedAudioFormats = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".flac": true,
	".aac":  true,
	".ogg":  true,
	".wma":  true,
	".m4a":  true,
}

// SupportedDocumentFormats 支持的文档格式
var SupportedDocumentFormats = map[string]bool{
	".pdf":  true,
	".doc":  true,
	".docx": true,
	".xls":  true,
	".xlsx": true,
	".ppt":  true,
	".pptx": true,
	".odt":  true,
	".ods":  true,
	".odp":  true,
}

// SupportedCodeFormats 支持的代码格式
var SupportedCodeFormats = map[string]bool{
	".go":    true,
	".js":    true,
	".ts":    true,
	".jsx":   true,
	".tsx":   true,
	".vue":   true,
	".py":    true,
	".java":  true,
	".c":     true,
	".cpp":   true,
	".h":     true,
	".hpp":   true,
	".cs":    true,
	".rs":    true,
	".rb":    true,
	".php":   true,
	".swift": true,
	".kt":    true,
	".scala": true,
	".sh":    true,
	".bash":  true,
	".zsh":   true,
	".fish":  true,
	".ps1":   true,
	".sql":   true,
	".html":  true,
	".css":   true,
	".scss":  true,
	".sass":  true,
	".less":  true,
	".json":  true,
	".xml":   true,
	".yaml":  true,
	".yml":   true,
	".toml":  true,
	".ini":   true,
	".conf":  true,
	".md":    true,
	".markdown": true,
}

// SupportedTextFormats 支持的纯文本格式
var SupportedTextFormats = map[string]bool{
	".txt":     true,
	".log":     true,
	".csv":     true,
	".env":     true,
	".gitignore": true,
	".dockerignore": true,
}
