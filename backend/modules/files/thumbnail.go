// Package files 缩略图生成服务
package files

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/image/draw"
)

// ThumbnailSize 缩略图尺寸
type ThumbnailSize int

const (
	ThumbnailSmall  ThumbnailSize = 128
	ThumbnailMedium ThumbnailSize = 256
	ThumbnailLarge  ThumbnailSize = 512
	ThumbnailXLarge ThumbnailSize = 1024
)

// ThumbnailService 缩略图服务
type ThumbnailService struct {
	logger    *zap.Logger
	cacheDir  string
	semaphore chan struct{} // 控制并发生成数量
	cache     sync.Map      // 内存缓存：path+size -> cachePath
}

// ThumbnailResult 缩略图结果
type ThumbnailResult struct {
	Path      string
	Data      []byte
	MimeType  string
	Width     int
	Height    int
	FromCache bool
}

// NewThumbnailService 创建缩略图服务
func NewThumbnailService(logger *zap.Logger, cacheDir string) *ThumbnailService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cacheDir == "" {
		cacheDir = "/var/cache/rde/thumbnails"
	}

	// 确保缓存目录存在
	_ = os.MkdirAll(cacheDir, 0755)

	return &ThumbnailService{
		logger:    logger,
		cacheDir:  cacheDir,
		semaphore: make(chan struct{}, 4), // 最多 4 个并发生成
	}
}

// GetThumbnail 获取缩略图
func (s *ThumbnailService) GetThumbnail(filePath string, size ThumbnailSize) (*ThumbnailResult, error) {
	// 验证文件存在
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// 计算缓存键
	cacheKey := s.getCacheKey(filePath, size, info.ModTime())
	cachePath := filepath.Join(s.cacheDir, cacheKey+".jpg")

	// 检查缓存
	if data, err := os.ReadFile(cachePath); err == nil {
		s.logger.Debug("thumbnail cache hit", zap.String("path", filePath))
		return &ThumbnailResult{
			Path:      cachePath,
			Data:      data,
			MimeType:  "image/jpeg",
			FromCache: true,
		}, nil
	}

	// 判断文件类型并生成缩略图
	ext := strings.ToLower(filepath.Ext(filePath))
	var result *ThumbnailResult

	if isImageExt(ext) {
		result, err = s.generateImageThumbnail(filePath, size)
	} else if isVideoExt(ext) {
		result, err = s.generateVideoThumbnail(filePath, size)
	} else {
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}

	if err != nil {
		return nil, err
	}

	// 保存到缓存
	if err := os.WriteFile(cachePath, result.Data, 0644); err != nil {
		s.logger.Warn("failed to save thumbnail cache", zap.Error(err))
	} else {
		s.cache.Store(filePath+fmt.Sprint(size), cachePath)
	}

	result.Path = cachePath
	return result, nil
}

// getCacheKey 生成缓存键
func (s *ThumbnailService) getCacheKey(filePath string, size ThumbnailSize, modTime time.Time) string {
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s:%d:%d", filePath, size, modTime.Unix())))
	return hex.EncodeToString(h.Sum(nil))
}

// generateImageThumbnail 生成图片缩略图
func (s *ThumbnailService) generateImageThumbnail(filePath string, size ThumbnailSize) (*ThumbnailResult, error) {
	// 获取信号量
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// 打开图片
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// 解码图片
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image (%s): %w", format, err)
	}

	// 计算缩放尺寸
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()
	targetSize := int(size)

	newWidth, newHeight := calculateThumbnailSize(origWidth, origHeight, targetSize)

	// 创建缩略图
	thumb := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), img, bounds, draw.Over, nil)

	// 编码为 JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return &ThumbnailResult{
		Data:     buf.Bytes(),
		MimeType: "image/jpeg",
		Width:    newWidth,
		Height:   newHeight,
	}, nil
}

// generateVideoThumbnail 生成视频缩略图（使用 ffmpeg）
func (s *ThumbnailService) generateVideoThumbnail(filePath string, size ThumbnailSize) (*ThumbnailResult, error) {
	// 获取信号量
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// 检查 ffmpeg 是否可用
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %w", err)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "thumb_*.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	targetSize := int(size)

	// 使用 ffmpeg 提取帧
	// -ss 1: 跳过第一秒（避免黑帧）
	// -vframes 1: 只提取一帧
	// -vf scale: 缩放
	cmd := exec.Command("ffmpeg",
		"-ss", "1",
		"-i", filePath,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':min'(%d,ih)':force_original_aspect_ratio=decrease", targetSize, targetSize),
		"-q:v", "2",
		"-y",
		tmpPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Warn("ffmpeg failed", zap.String("output", string(output)), zap.Error(err))
		// 尝试从开头提取
		cmd = exec.Command("ffmpeg",
			"-i", filePath,
			"-vframes", "1",
			"-vf", fmt.Sprintf("scale='min(%d,iw)':min'(%d,ih)':force_original_aspect_ratio=decrease", targetSize, targetSize),
			"-q:v", "2",
			"-y",
			tmpPath,
		)
		if output, err = cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("ffmpeg failed: %s - %w", string(output), err)
		}
	}

	// 读取生成的缩略图
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail: %w", err)
	}

	return &ThumbnailResult{
		Data:     data,
		MimeType: "image/jpeg",
	}, nil
}

// calculateThumbnailSize 计算缩略图尺寸（保持宽高比）
func calculateThumbnailSize(origWidth, origHeight, targetSize int) (int, int) {
	if origWidth <= targetSize && origHeight <= targetSize {
		return origWidth, origHeight
	}

	var newWidth, newHeight int
	if origWidth > origHeight {
		newWidth = targetSize
		newHeight = origHeight * targetSize / origWidth
	} else {
		newHeight = targetSize
		newWidth = origWidth * targetSize / origHeight
	}

	if newWidth == 0 {
		newWidth = 1
	}
	if newHeight == 0 {
		newHeight = 1
	}

	return newWidth, newHeight
}

// isImageExt 判断是否为图片扩展名
func isImageExt(ext string) bool {
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true, ".ico": true, ".tiff": true,
	}
	return imageExts[ext]
}

// isVideoExt 判断是否为视频扩展名
func isVideoExt(ext string) bool {
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".3gp": true, ".ts": true, ".m2ts": true,
	}
	return videoExts[ext]
}

// ClearCache 清理过期缓存
func (s *ThumbnailService) ClearCache(maxAge time.Duration) error {
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			os.Remove(filepath.Join(s.cacheDir, entry.Name()))
		}
	}
	return nil
}

// SupportsThumbnail 判断文件是否支持缩略图
func SupportsThumbnail(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return isImageExt(ext) || isVideoExt(ext)
}

// 注册额外的图片格式解码器
func init() {
	// 标准库已支持 jpeg, png, gif
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "\x89PNG\r\n\x1a\n", png.Decode, png.DecodeConfig)
	image.RegisterFormat("gif", "GIF8?a", gif.Decode, gif.DecodeConfig)
}

// GetThumbnailReader 获取缩略图 Reader（用于流式传输）
func (s *ThumbnailService) GetThumbnailReader(filePath string, size ThumbnailSize) (io.ReadCloser, string, error) {
	result, err := s.GetThumbnail(filePath, size)
	if err != nil {
		return nil, "", err
	}
	return io.NopCloser(bytes.NewReader(result.Data)), result.MimeType, nil
}
