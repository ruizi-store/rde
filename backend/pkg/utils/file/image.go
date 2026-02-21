package file

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
)

// ImageInfo 图片信息
type ImageInfo struct {
	Width  int
	Height int
	Format string
}

// GetImageInfo 获取图片信息
func GetImageInfo(path string) (*ImageInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return nil, err
	}

	return &ImageInfo{
		Width:  config.Width,
		Height: config.Height,
		Format: format,
	}, nil
}

// IsImage 检查文件是否为图片
func IsImage(path string) bool {
	ext := strings.ToLower(GetExtension(path))
	switch ext {
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "ico", "tiff", "tif":
		return true
	}
	return false
}

// IsImageByMagic 通过魔数检查文件是否为图片
func IsImageByMagic(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// 读取前 16 字节
	buf := make([]byte, 16)
	_, err = file.Read(buf)
	if err != nil {
		return false
	}

	return detectImageFormat(buf) != ""
}

// detectImageFormat 通过魔数检测图片格式
func detectImageFormat(data []byte) string {
	// JPEG: FF D8 FF
	if len(data) >= 3 && bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "jpeg"
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(data) >= 8 && bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "png"
	}
	// GIF: GIF87a or GIF89a
	if len(data) >= 6 && (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))) {
		return "gif"
	}
	// BMP: BM
	if len(data) >= 2 && bytes.HasPrefix(data, []byte{0x42, 0x4D}) {
		return "bmp"
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "webp"
	}
	// ICO: 00 00 01 00
	if len(data) >= 4 && bytes.HasPrefix(data, []byte{0x00, 0x00, 0x01, 0x00}) {
		return "ico"
	}
	return ""
}

// GetImageDimensions 获取图片尺寸
func GetImageDimensions(path string) (width, height int, err error) {
	info, err := GetImageInfo(path)
	if err != nil {
		return 0, 0, err
	}
	return info.Width, info.Height, nil
}

// ImageAspectRatio 计算图片宽高比
func ImageAspectRatio(width, height int) string {
	if height == 0 {
		return "0:0"
	}
	
	// 简化比例
	gcd := gcdFunc(width, height)
	return fmt.Sprintf("%d:%d", width/gcd, height/gcd)
}

// gcdFunc 计算最大公约数
func gcdFunc(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// IsVideo 检查文件是否为视频
func IsVideo(path string) bool {
	ext := strings.ToLower(GetExtension(path))
	switch ext {
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpeg", "mpg", "3gp":
		return true
	}
	return false
}

// IsAudio 检查文件是否为音频
func IsAudio(path string) bool {
	ext := strings.ToLower(GetExtension(path))
	switch ext {
	case "mp3", "wav", "flac", "aac", "ogg", "wma", "m4a", "opus", "ape":
		return true
	}
	return false
}

// IsDocument 检查文件是否为文档
func IsDocument(path string) bool {
	ext := strings.ToLower(GetExtension(path))
	switch ext {
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "md", "rtf", "odt", "ods", "odp":
		return true
	}
	return false
}

// IsArchive 检查文件是否为压缩包
func IsArchive(path string) bool {
	ext := strings.ToLower(GetExtension(path))
	switch ext {
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "tgz", "tbz2":
		return true
	}
	return false
}

// GetMimeType 获取文件 MIME 类型（简单版本）
func GetMimeType(path string) string {
	ext := strings.ToLower(GetExtension(path))
	
	mimeTypes := map[string]string{
		// 图片
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"png":  "image/png",
		"gif":  "image/gif",
		"bmp":  "image/bmp",
		"webp": "image/webp",
		"ico":  "image/x-icon",
		"svg":  "image/svg+xml",
		// 视频
		"mp4":  "video/mp4",
		"mkv":  "video/x-matroska",
		"avi":  "video/x-msvideo",
		"mov":  "video/quicktime",
		"webm": "video/webm",
		// 音频
		"mp3":  "audio/mpeg",
		"wav":  "audio/wav",
		"flac": "audio/flac",
		"ogg":  "audio/ogg",
		// 文档
		"pdf":  "application/pdf",
		"doc":  "application/msword",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xls":  "application/vnd.ms-excel",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"ppt":  "application/vnd.ms-powerpoint",
		"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"txt":  "text/plain",
		"md":   "text/markdown",
		"html": "text/html",
		"css":  "text/css",
		"js":   "application/javascript",
		"json": "application/json",
		"xml":  "application/xml",
		// 压缩
		"zip": "application/zip",
		"rar": "application/x-rar-compressed",
		"7z":  "application/x-7z-compressed",
		"tar": "application/x-tar",
		"gz":  "application/gzip",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}
