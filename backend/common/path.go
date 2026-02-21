package common

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// 全局配置
var (
	// 允许访问的根目录列表
	allowedRoots   = []string{"/"}
	allowedRootsMu sync.RWMutex

	// 敏感目录（禁止访问）
	sensitiveRoots = []string{"/proc", "/sys", "/dev", "/run"}
)

// SetAllowedRoots 设置允许访问的根目录列表
func SetAllowedRoots(roots []string) {
	allowedRootsMu.Lock()
	defer allowedRootsMu.Unlock()
	allowedRoots = roots
}

// GetAllowedRoots 获取允许访问的根目录列表
func GetAllowedRoots() []string {
	allowedRootsMu.RLock()
	defer allowedRootsMu.RUnlock()
	result := make([]string, len(allowedRoots))
	copy(result, allowedRoots)
	return result
}

// DataRootResolver 数据根目录解析器（保留兼容）
type DataRootResolver interface {
	GetDataRoot() string
}

var (
	dataRootResolver DataRootResolver
	resolverMutex    sync.RWMutex
)

// SetDataRootResolver 设置数据根目录解析器
func SetDataRootResolver(resolver DataRootResolver) {
	resolverMutex.Lock()
	defer resolverMutex.Unlock()
	dataRootResolver = resolver
}

// GetDataRootPath 获取用户存储根目录（保留兼容）
func GetDataRootPath() string {
	resolverMutex.RLock()
	resolver := dataRootResolver
	resolverMutex.RUnlock()

	if resolver != nil {
		if path := resolver.GetDataRoot(); path != "" {
			return path
		}
	}

	sysType := runtime.GOOS
	switch sysType {
	case "windows":
		return "C:\\RDE\\Storage"
	default:
		defaultPath := "/var/lib/rde/data"
		if err := os.MkdirAll(defaultPath, 0755); err == nil {
			return defaultPath
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "/tmp/rde/storage"
		}
		storagePath := filepath.Join(home, ".rde", "storage")
		os.MkdirAll(storagePath, 0755)
		return storagePath
	}
}

// GetVolumesPath 获取挂载卷路径
func GetVolumesPath() string {
	sysType := runtime.GOOS
	switch sysType {
	case "windows":
		return "C:\\"
	case "darwin":
		return "/Volumes"
	default:
		return "/mnt"
	}
}

// VirtualPathContext 路径解析上下文（保留兼容，但不再用于虚拟路径转换）
type VirtualPathContext struct {
	Username string
	IsAdmin  bool
}

// ResolvePath 解析并清理路径，返回安全的绝对路径
func ResolvePath(path string) string {
	// 清理路径，去除 . 和 ..
	cleaned := filepath.Clean(path)

	// 确保是绝对路径
	if !filepath.IsAbs(cleaned) {
		cleaned = "/" + cleaned
	}

	return cleaned
}

// ResolveVirtualPath 解析路径（保留兼容，现在直接返回真实路径）
func ResolveVirtualPath(path string) string {
	return ResolvePath(path)
}

// ResolveVirtualPathWithContext 解析路径（保留兼容）
func ResolveVirtualPathWithContext(path string, ctx *VirtualPathContext) string {
	return ResolvePath(path)
}

// ToVirtualPath 转换路径（保留兼容，现在直接返回原路径）
func ToVirtualPath(realPath string) string {
	return realPath
}

// ToVirtualPathWithContext 转换路径（保留兼容）
func ToVirtualPathWithContext(realPath string, ctx *VirtualPathContext) string {
	return realPath
}

// IsSensitivePath 检查路径是否为系统敏感目录（/proc, /sys, /dev, /run）
// 敏感目录需要管理员提权后才能访问
func IsSensitivePath(path string) bool {
	cleaned := ResolvePath(path)
	for _, sensitive := range sensitiveRoots {
		if cleaned == sensitive || strings.HasPrefix(cleaned, sensitive+"/") {
			return true
		}
	}
	return false
}

// IsPathAllowed 检查路径是否允许访问（不包含敏感目录检查，敏感目录由 handler 层处理）
func IsPathAllowed(path string, ctx *VirtualPathContext) bool {
	cleaned := ResolvePath(path)

	// 检查是否在允许的根目录内
	allowedRootsMu.RLock()
	roots := allowedRoots
	allowedRootsMu.RUnlock()

	for _, root := range roots {
		if root == "/" {
			return true
		}
		if cleaned == root || strings.HasPrefix(cleaned, root+"/") {
			return true
		}
	}

	return false
}

// ListVirtualRoot 列出允许的根目录（保留兼容）
func ListVirtualRoot(ctx *VirtualPathContext) []string {
	return GetAllowedRoots()
}
