// Package config 提供配置管理服务
package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
)

// Config 配置管理器
type Config struct {
	mu    sync.RWMutex
	viper *viper.Viper

	// 常用路径
	DataPath   string // 数据目录
	DBPath     string // 数据库路径
	LogPath    string // 日志目录
	CachePath  string // 缓存目录
	ConfigPath string // 配置文件路径
}

// New 创建配置管理器
func New() *Config {
	v := viper.New()
	v.SetConfigType("toml")

	// 设置默认值
	setDefaults(v)

	c := &Config{
		viper: v,
	}

	// 初始化路径
	c.initPaths()

	return c
}

// Load 从文件加载配置
func (c *Config) Load(configPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if configPath != "" {
		c.viper.SetConfigFile(configPath)
		c.ConfigPath = configPath
	}

	if err := c.viper.ReadInConfig(); err != nil {
		// 配置文件不存在不是错误
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	// 重新初始化路径
	c.initPaths()
	return nil
}

// initPaths 初始化各种路径
func (c *Config) initPaths() {
	homeDir, _ := os.UserHomeDir()
	defaultBase := filepath.Join(homeDir, ".rde")

	c.DataPath = c.viper.GetString("paths.data")
	if c.DataPath == "" {
		c.DataPath = filepath.Join(defaultBase, "data")
	}

	c.DBPath = c.viper.GetString("paths.db")
	if c.DBPath == "" {
		c.DBPath = filepath.Join(defaultBase, "db", "rde.db")
	}

	c.LogPath = c.viper.GetString("paths.log")
	if c.LogPath == "" {
		c.LogPath = filepath.Join(defaultBase, "logs")
	}

	c.CachePath = c.viper.GetString("paths.cache")
	if c.CachePath == "" {
		c.CachePath = filepath.Join(defaultBase, "cache")
	}

	// 确保目录存在
	os.MkdirAll(c.DataPath, 0755)
	os.MkdirAll(filepath.Dir(c.DBPath), 0755)
	os.MkdirAll(c.LogPath, 0755)
	os.MkdirAll(c.CachePath, 0755)
}

// setDefaults 设置默认配置
func setDefaults(v *viper.Viper) {
	// 服务器配置
	v.SetDefault("server.port", 3080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.debug", false)

	// JWT 配置
	v.SetDefault("auth.jwt_secret", "rde-default-secret-change-me")
	v.SetDefault("auth.jwt_expire", 86400*7) // 7 天

	// 文件配置
	v.SetDefault("files.max_upload_size", 10*1024*1024*1024) // 10GB
	v.SetDefault("files.thumbnail_size", 200)

	// AI 配置
	v.SetDefault("ai.enabled", true)
	v.SetDefault("ai.provider", "openai")

	// 插件配置
	v.SetDefault("plugins.disabled", []string{})
}

// Get 获取配置值
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.Get(key)
}

// GetString 获取字符串配置
func (c *Config) GetString(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 特殊处理已解析的路径字段
	switch key {
	case "data_path", "paths.data":
		return c.DataPath
	case "db_path", "paths.db":
		return c.DBPath
	case "log_path", "paths.log":
		return c.LogPath
	case "cache_path", "paths.cache":
		return c.CachePath
	}

	return c.viper.GetString(key)
}

// GetInt 获取整数配置
func (c *Config) GetInt(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.GetInt(key)
}

// GetInt64 获取 int64 配置
func (c *Config) GetInt64(key string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.GetInt64(key)
}

// GetBool 获取布尔配置
func (c *Config) GetBool(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.GetBool(key)
}

// GetStringSlice 获取字符串数组配置
func (c *Config) GetStringSlice(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.GetStringSlice(key)
}

// GetStringMap 获取字符串 map 配置
func (c *Config) GetStringMap(key string) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.GetStringMap(key)
}

// Set 设置配置值（运行时）
func (c *Config) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.viper.Set(key, value)
}

// IsSet 检查配置是否已设置
func (c *Config) IsSet(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.IsSet(key)
}

// AllSettings 获取所有配置
func (c *Config) AllSettings() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.AllSettings()
}

// Sub 获取子配置
func (c *Config) Sub(key string) *viper.Viper {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.viper.Sub(key)
}

// Save 将当前配置持久化到文件
// 如果配置文件路径尚未设定，则写入默认位置
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ConfigPath != "" {
		return c.viper.WriteConfigAs(c.ConfigPath)
	}
	return c.viper.WriteConfig()
}
