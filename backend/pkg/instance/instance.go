// Package instance 提供安装实例唯一标识（重装/新机必变），供前后端对账清浏览器缓存。
package instance

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	DefaultDataDir = "/var/lib/rde"
	FileName       = "instance-id"
	HeaderName     = "X-RDE-Instance-Id"
)

var (
	mu     sync.RWMutex
	cached string
	dir    string
)

// Init 从 dataDir/instance-id 加载或创建 UUID，可重复调用以切换目录（测试）。
func Init(dataDir string) (string, error) {
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	mu.Lock()
	defer mu.Unlock()

	dir = dataDir
	path := filepath.Join(dataDir, FileName)

	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			cached = id
			return cached, nil
		}
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", err
	}
	id := uuid.NewString()
	if err := os.WriteFile(path, []byte(id+"\n"), 0o644); err != nil {
		return "", err
	}
	cached = id
	return cached, nil
}

// ID 返回当前实例 ID（未 Init 时尝试默认目录）。
func ID() string {
	mu.RLock()
	if cached != "" {
		defer mu.RUnlock()
		return cached
	}
	mu.RUnlock()

	id, err := Init(DefaultDataDir)
	if err != nil {
		return ""
	}
	return id
}

// BootID 读取内核 boot_id（可选，重启会变；同机重装仍可能相同直到 reboot）。
func BootID() string {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
