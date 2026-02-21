// Package photos 照片管理模块
package photos

import "time"

// Photo 照片记录
type Photo struct {
	ID        string     `json:"id" gorm:"primaryKey;size:36"`
	LibraryID string     `json:"library_id" gorm:"index;size:36"`
	Path      string     `json:"path" gorm:"uniqueIndex;size:1024"`
	Filename  string     `json:"filename" gorm:"size:255"`
	Hash      string     `json:"hash" gorm:"index;size:64"` // SHA256 去重
	Size      int64      `json:"size"`
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	MimeType  string     `json:"mime_type" gorm:"size:64"`
	Type      string     `json:"type" gorm:"size:16"` // photo, video
	TakenAt   *time.Time `json:"taken_at" gorm:"index"`
	Timezone  string     `json:"timezone" gorm:"size:64"`
	Duration  float64    `json:"duration"` // 视频时长(秒)

	// EXIF 数据
	CameraMake   string  `json:"camera_make" gorm:"size:64"`
	CameraModel  string  `json:"camera_model" gorm:"size:64"`
	LensMake     string  `json:"lens_make" gorm:"size:64"`
	LensModel    string  `json:"lens_model" gorm:"size:128"`
	FNumber      float64 `json:"f_number"`
	ExposureTime string  `json:"exposure_time" gorm:"size:32"`
	ISO          int     `json:"iso"`
	FocalLength  float64 `json:"focal_length"`
	Orientation  int     `json:"orientation"`

	// GPS 位置
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Altitude  *float64 `json:"altitude"`
	City      string   `json:"city" gorm:"size:128"`
	Country   string   `json:"country" gorm:"size:64"`

	// 状态
	Status     string     `json:"status" gorm:"size:16;default:pending"` // pending, indexed, failed
	IsFavorite bool       `json:"is_favorite" gorm:"index"`
	IsArchived bool       `json:"is_archived" gorm:"index"`
	IsDeleted  bool       `json:"is_deleted" gorm:"index"`
	DeletedAt  *time.Time `json:"deleted_at"`

	// AI 分析结果 (来自 indexer 服务)
	AIIndexed bool    `json:"ai_indexed" gorm:"index;default:false"`
	HasFaces  bool    `json:"has_faces" gorm:"index;default:false"`
	FaceCount int     `json:"face_count" gorm:"default:0"`
	OCRText   string  `json:"ocr_text,omitempty" gorm:"type:text"`

	// AI 搜索时的临时字段 (不存储)
	AIScore  float64 `json:"ai_score,omitempty" gorm:"-"`
	AIAge    *int    `json:"ai_age,omitempty" gorm:"-"`
	AIGender *string `json:"ai_gender,omitempty" gorm:"-"`

	// 时间戳
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	IndexedAt *time.Time `json:"indexed_at"`

	// 关联
	Library *Library `json:"library,omitempty" gorm:"foreignKey:LibraryID"`
}

func (Photo) TableName() string {
	return "photos"
}

// Album 相册
type Album struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	Name        string    `json:"name" gorm:"size:255"`
	Description string    `json:"description" gorm:"size:1024"`
	CoverID     string    `json:"cover_id" gorm:"size:36"`
	Type        string    `json:"type" gorm:"size:16;default:manual"` // manual, auto, shared
	SortOrder   string    `json:"sort_order" gorm:"size:16;default:date_desc"`
	UserID      string    `json:"user_id" gorm:"index;size:36"`
	IsPublic    bool      `json:"is_public"`
	ShareToken  string    `json:"share_token,omitempty" gorm:"size:64;index"`
	PhotoCount  int       `json:"photo_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Photos []Photo `json:"photos,omitempty" gorm:"many2many:album_photos"`
}

func (Album) TableName() string {
	return "albums"
}

// AlbumPhoto 相册-照片关联
type AlbumPhoto struct {
	AlbumID   string    `json:"album_id" gorm:"primaryKey;size:36"`
	PhotoID   string    `json:"photo_id" gorm:"primaryKey;size:36"`
	SortOrder int       `json:"sort_order"`
	AddedAt   time.Time `json:"added_at"`
}

func (AlbumPhoto) TableName() string {
	return "album_photos"
}

// Library 图库（监控目录）
type Library struct {
	ID          string     `json:"id" gorm:"primaryKey;size:36"`
	Name        string     `json:"name" gorm:"size:255"`
	Path        string     `json:"path" gorm:"uniqueIndex;size:1024"`
	UserID      string     `json:"user_id" gorm:"index;size:36"`
	ScanEnabled bool       `json:"scan_enabled" gorm:"default:true"`
	LastScanAt  *time.Time `json:"last_scan_at"`
	PhotoCount  int        `json:"photo_count"`
	VideoCount  int        `json:"video_count"`
	TotalSize   int64      `json:"total_size"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Library) TableName() string {
	return "libraries"
}
