// Package photos 照片管理模块
package photos

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Indexer 元数据索引器
type Indexer struct {
	logger  *zap.Logger
	db      *gorm.DB
	service *Service
}

// NewIndexer 创建索引器
func NewIndexer(logger *zap.Logger, db *gorm.DB, service *Service) *Indexer {
	return &Indexer{
		logger:  logger,
		db:      db,
		service: service,
	}
}

// IndexPhoto 索引照片
func (i *Indexer) IndexPhoto(photo *Photo) error {
	var err error

	switch photo.Type {
	case "photo":
		err = i.indexImage(photo)
	case "video":
		err = i.indexVideo(photo)
	default:
		err = i.indexImage(photo)
	}

	// 更新状态
	now := time.Now()
	if err != nil {
		photo.Status = "failed"
		i.logger.Debug("failed to index photo",
			zap.String("id", photo.ID),
			zap.String("path", photo.Path),
			zap.Error(err))
	} else {
		photo.Status = "indexed"
		photo.IndexedAt = &now
	}
	photo.UpdatedAt = now

	return i.db.Save(photo).Error
}

// indexImage 索引图片
func (i *Indexer) indexImage(photo *Photo) error {
	f, err := os.Open(photo.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	// 计算文件哈希
	if photo.Hash == "" {
		hash, err := CalculateFileHash(photo.Path)
		if err == nil {
			photo.Hash = hash
		}
	}

	// 尝试提取 EXIF
	x, err := exif.Decode(f)
	if err != nil {
		// 没有 EXIF 数据也不算错误
		i.logger.Debug("no exif data", zap.String("path", photo.Path))
		return i.extractImageDimensions(photo)
	}

	// 提取拍摄时间
	if dt, err := x.DateTime(); err == nil {
		photo.TakenAt = &dt
	}

	// 提取相机信息
	if make, err := x.Get(exif.Make); err == nil {
		photo.CameraMake = strings.TrimSpace(make.String())
	}
	if model, err := x.Get(exif.Model); err == nil {
		photo.CameraModel = strings.TrimSpace(model.String())
	}

	// 提取镜头信息
	if lensMake, err := x.Get(exif.LensMake); err == nil {
		photo.LensMake = strings.TrimSpace(lensMake.String())
	}
	if lensModel, err := x.Get(exif.LensModel); err == nil {
		photo.LensModel = strings.TrimSpace(lensModel.String())
	}

	// 提取拍摄参数
	if fNumber, err := x.Get(exif.FNumber); err == nil {
		if rat, err := fNumber.Rat(0); err == nil {
			f, _ := rat.Float64()
			photo.FNumber = f
		}
	}
	if exposure, err := x.Get(exif.ExposureTime); err == nil {
		photo.ExposureTime = exposure.String()
	}
	if iso, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if val, err := iso.Int(0); err == nil {
			photo.ISO = val
		}
	}
	if focal, err := x.Get(exif.FocalLength); err == nil {
		if rat, err := focal.Rat(0); err == nil {
			f, _ := rat.Float64()
			photo.FocalLength = f
		}
	}

	// 提取方向
	if orientation, err := x.Get(exif.Orientation); err == nil {
		if val, err := orientation.Int(0); err == nil {
			photo.Orientation = val
		}
	}

	// 提取 GPS 坐标
	if lat, lon, err := x.LatLong(); err == nil {
		photo.Latitude = &lat
		photo.Longitude = &lon
	}
	if alt, err := x.Get(exif.GPSAltitude); err == nil {
		if rat, err := alt.Rat(0); err == nil {
			f, _ := rat.Float64()
			photo.Altitude = &f
		}
	}

	// 提取图片尺寸
	if width, err := x.Get(exif.PixelXDimension); err == nil {
		if val, err := width.Int(0); err == nil {
			photo.Width = val
		}
	}
	if height, err := x.Get(exif.PixelYDimension); err == nil {
		if val, err := height.Int(0); err == nil {
			photo.Height = val
		}
	}

	// 如果没有从 EXIF 获取到尺寸，使用其他方式
	if photo.Width == 0 || photo.Height == 0 {
		if err := i.extractImageDimensions(photo); err != nil {
			i.logger.Debug("failed to extract dimensions", zap.String("path", photo.Path), zap.Error(err))
		}
	}

	// 获取 MIME 类型
	if photo.MimeType == "" {
		photo.MimeType = getMimeType(photo.Path)
	}

	return nil
}

// extractImageDimensions 使用 identify 命令提取图片尺寸
func (i *Indexer) extractImageDimensions(photo *Photo) error {
	// 尝试使用 ImageMagick 的 identify
	cmd := exec.Command("identify", "-format", "%w %h", photo.Path)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		if w, err := strconv.Atoi(parts[0]); err == nil {
			photo.Width = w
		}
		if h, err := strconv.Atoi(parts[1]); err == nil {
			photo.Height = h
		}
	}

	return nil
}

// indexVideo 索引视频
func (i *Indexer) indexVideo(photo *Photo) error {
	// 计算文件哈希
	if photo.Hash == "" {
		hash, err := CalculateFileHash(photo.Path)
		if err == nil {
			photo.Hash = hash
		}
	}

	// 使用 ffprobe 提取视频元数据
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		photo.Path,
	)

	output, err := cmd.Output()
	if err != nil {
		i.logger.Debug("ffprobe failed", zap.String("path", photo.Path), zap.Error(err))
		// 尝试从文件名或修改时间获取信息
		return i.extractVideoBasicInfo(photo)
	}

	// 解析 ffprobe 输出
	return i.parseFFprobeOutput(photo, output)
}

// extractVideoBasicInfo 提取视频基本信息
func (i *Indexer) extractVideoBasicInfo(photo *Photo) error {
	info, err := os.Stat(photo.Path)
	if err != nil {
		return err
	}

	// 使用文件修改时间作为拍摄时间
	modTime := info.ModTime()
	photo.TakenAt = &modTime
	photo.MimeType = getMimeType(photo.Path)

	return nil
}

// parseFFprobeOutput 解析 ffprobe 输出
func (i *Indexer) parseFFprobeOutput(photo *Photo, output []byte) error {
	// 简化处理：使用 ffprobe 的简单输出格式
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,duration",
		"-of", "csv=p=0",
		photo.Path,
	)

	out, err := cmd.Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(out)), ",")
		if len(parts) >= 2 {
			if w, err := strconv.Atoi(parts[0]); err == nil {
				photo.Width = w
			}
			if h, err := strconv.Atoi(parts[1]); err == nil {
				photo.Height = h
			}
		}
		if len(parts) >= 3 {
			if d, err := strconv.ParseFloat(parts[2], 64); err == nil {
				photo.Duration = d
			}
		}
	}

	// 获取创建时间
	cmd = exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format_tags=creation_time",
		"-of", "csv=p=0",
		photo.Path,
	)

	out, err = cmd.Output()
	if err == nil {
		timeStr := strings.TrimSpace(string(out))
		if timeStr != "" {
			// 尝试解析多种时间格式
			formats := []string{
				"2006-01-02T15:04:05.000000Z",
				"2006-01-02T15:04:05Z",
				"2006-01-02 15:04:05",
			}
			for _, format := range formats {
				if t, err := time.Parse(format, timeStr); err == nil {
					photo.TakenAt = &t
					break
				}
			}
		}
	}

	// 如果没有拍摄时间，使用文件修改时间
	if photo.TakenAt == nil {
		info, err := os.Stat(photo.Path)
		if err == nil {
			modTime := info.ModTime()
			photo.TakenAt = &modTime
		}
	}

	photo.MimeType = getMimeType(photo.Path)
	return nil
}

// BatchIndex 批量索引
func (i *Indexer) BatchIndex(photoIDs []string) error {
	var photos []Photo
	if err := i.db.Where("id IN ? AND status = ?", photoIDs, "pending").Find(&photos).Error; err != nil {
		return err
	}

	for _, photo := range photos {
		if err := i.IndexPhoto(&photo); err != nil {
			i.logger.Debug("failed to index photo", zap.String("id", photo.ID), zap.Error(err))
		}
	}

	return nil
}

// ReindexLibrary 重新索引图库
func (i *Indexer) ReindexLibrary(libraryID string) error {
	// 重置所有照片状态为 pending
	if err := i.db.Model(&Photo{}).Where("library_id = ?", libraryID).
		Update("status", "pending").Error; err != nil {
		return err
	}

	// 获取所有照片
	var photos []Photo
	if err := i.db.Where("library_id = ?", libraryID).Find(&photos).Error; err != nil {
		return err
	}

	for _, photo := range photos {
		if err := i.IndexPhoto(&photo); err != nil {
			i.logger.Debug("failed to reindex photo", zap.String("id", photo.ID), zap.Error(err))
		}
	}

	return nil
}

// getMimeType 获取 MIME 类型
func getMimeType(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(path[strings.LastIndex(path, "."):], "."))
	mimeTypes := map[string]string{
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"png":  "image/png",
		"gif":  "image/gif",
		"webp": "image/webp",
		"bmp":  "image/bmp",
		"tiff": "image/tiff",
		"tif":  "image/tiff",
		"heic": "image/heic",
		"heif": "image/heif",
		"raw":  "image/x-raw",
		"cr2":  "image/x-canon-cr2",
		"nef":  "image/x-nikon-nef",
		"arw":  "image/x-sony-arw",
		"dng":  "image/x-adobe-dng",
		"mp4":  "video/mp4",
		"mov":  "video/quicktime",
		"avi":  "video/x-msvideo",
		"mkv":  "video/x-matroska",
		"wmv":  "video/x-ms-wmv",
		"flv":  "video/x-flv",
		"webm": "video/webm",
		"m4v":  "video/x-m4v",
		"3gp":  "video/3gpp",
		"mts":  "video/mp2t",
		"m2ts": "video/mp2t",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// GenerateThumbnails 生成缩略图
func (i *Indexer) GenerateThumbnails(photo *Photo) error {
	// 由 ThumbnailService 处理，这里只是触发
	// 实际的缩略图生成在请求时按需进行
	return nil
}

// CalculatePlaceholderID 生成占位符 ID（用于去重）
func CalculatePlaceholderID(hash string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(hash)).String()
}
