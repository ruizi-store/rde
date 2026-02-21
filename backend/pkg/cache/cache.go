package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Init 初始化缓存 - CasaOS 兼容
// 返回 patrickmn/go-cache 的实例
func Init() *gocache.Cache {
	return gocache.New(5*time.Minute, 10*time.Minute)
}

// New 创建新的缓存实例
func New() *gocache.Cache {
	return gocache.New(5*time.Minute, 10*time.Minute)
}

// NewWithExpiration 创建带过期时间的缓存实例
func NewWithExpiration(defaultExpiration, cleanupInterval time.Duration) *gocache.Cache {
	return gocache.New(defaultExpiration, cleanupInterval)
}
