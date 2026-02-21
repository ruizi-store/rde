package config

import (
	"os"
	"path/filepath"
)

// getDefaultDataPath 获取默认数据目录
func getDefaultDataPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/rde/data"
	}
	return filepath.Join(home, ".rde", "data")
}

// 默认配置路径
var (
	DefaultConfigPaths = []string{
		"/etc/rde/config.yaml",
		"/etc/rde/config.json",
		"./config.yaml",
		"./config.json",
		"./conf/config.yaml",
		"./conf/config.json",
	}
)

// AppConfig 应用配置结构
type AppConfig struct {
	// 服务器配置
	Server ServerConfig `json:"server" yaml:"server"`
	// 数据库配置
	Database DatabaseConfig `json:"database" yaml:"database"`
	// 日志配置
	Log LogConfig `json:"log" yaml:"log"`
	// JWT 配置
	JWT JWTConfig `json:"jwt" yaml:"jwt"`
	// Docker 配置
	Docker DockerConfig `json:"docker" yaml:"docker"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string `json:"host" yaml:"host"`
	Port         int    `json:"port" yaml:"port"`
	ReadTimeout  int    `json:"read_timeout" yaml:"read_timeout"`   // 秒
	WriteTimeout int    `json:"write_timeout" yaml:"write_timeout"` // 秒
	Debug        bool   `json:"debug" yaml:"debug"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type     string `json:"type" yaml:"type"` // sqlite, mysql, postgres
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
	Name     string `json:"name" yaml:"name"`
	Path     string `json:"path" yaml:"path"` // SQLite 数据库路径
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `json:"level" yaml:"level"`   // debug, info, warn, error
	Format string `json:"format" yaml:"format"` // text, json
	Output string `json:"output" yaml:"output"` // stdout, file path
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret        string `json:"secret" yaml:"secret"`
	AccessExpiry  int    `json:"access_expiry" yaml:"access_expiry"`   // 小时
	RefreshExpiry int    `json:"refresh_expiry" yaml:"refresh_expiry"` // 小时
}

// DockerConfig Docker 配置
type DockerConfig struct {
	Host      string `json:"host" yaml:"host"`
	Version   string `json:"version" yaml:"version"`
	TLSVerify bool   `json:"tls_verify" yaml:"tls_verify"`
	CertPath  string `json:"cert_path" yaml:"cert_path"`
}

// DefaultAppConfig 默认应用配置
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30,
			WriteTimeout: 30,
			Debug:        false,
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			Path: "/var/lib/rde/rde.db",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		JWT: JWTConfig{
			Secret:        "change-me-in-production",
			AccessExpiry:  24,
			RefreshExpiry: 168, // 7 天
		},
		Docker: DockerConfig{
			Host:      "unix:///var/run/docker.sock",
			TLSVerify: false,
		},
	}
}

// Init 初始化配置
func Init() (*AppConfig, error) {
	return InitWithPath("")
}

// InitWithPath 从指定路径初始化配置
func InitWithPath(configPath string) (*AppConfig, error) {
	cfg := Global()

	// 设置默认值
	setDefaults(cfg)

	// 加载配置文件
	loaded := false
	if configPath != "" {
		if err := cfg.Load(configPath); err == nil {
			loaded = true
		}
	}

	if !loaded {
		// 尝试默认路径
		for _, path := range DefaultConfigPaths {
			if _, err := os.Stat(path); err == nil {
				if err := cfg.Load(path); err == nil {
					loaded = true
					break
				}
			}
		}
	}

	// 从环境变量覆盖
	cfg.LoadEnv("RDE_")

	// 解析到结构体
	appConfig := DefaultAppConfig()
	if err := cfg.UnmarshalAll(appConfig); err != nil {
		// 使用默认配置
		return appConfig, nil
	}

	return appConfig, nil
}

// setDefaults 设置默认配置值
func setDefaults(cfg *Config) {
	defaults := DefaultAppConfig()

	cfg.SetDefault("server.host", defaults.Server.Host)
	cfg.SetDefault("server.port", defaults.Server.Port)
	cfg.SetDefault("server.read_timeout", defaults.Server.ReadTimeout)
	cfg.SetDefault("server.write_timeout", defaults.Server.WriteTimeout)
	cfg.SetDefault("server.debug", defaults.Server.Debug)

	cfg.SetDefault("database.type", defaults.Database.Type)
	cfg.SetDefault("database.path", defaults.Database.Path)

	cfg.SetDefault("log.level", defaults.Log.Level)
	cfg.SetDefault("log.format", defaults.Log.Format)
	cfg.SetDefault("log.output", defaults.Log.Output)

	cfg.SetDefault("jwt.secret", defaults.JWT.Secret)
	cfg.SetDefault("jwt.access_expiry", defaults.JWT.AccessExpiry)
	cfg.SetDefault("jwt.refresh_expiry", defaults.JWT.RefreshExpiry)

	cfg.SetDefault("docker.host", defaults.Docker.Host)
	cfg.SetDefault("docker.tls_verify", defaults.Docker.TLSVerify)
}

// EnsureDirectories 确保必要的目录存在
func EnsureDirectories(cfg *AppConfig) error {
	dirs := []string{
		filepath.Dir(cfg.Database.Path),
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// Validate 验证配置
func (c *AppConfig) Validate() error {
	// 验证端口范围
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		c.Server.Port = 8080
	}

	// 验证日志级别
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Log.Level] {
		c.Log.Level = "info"
	}

	// 验证数据库类型
	validDBTypes := map[string]bool{"sqlite": true, "mysql": true, "postgres": true}
	if !validDBTypes[c.Database.Type] {
		c.Database.Type = "sqlite"
	}

	return nil
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	switch c.Type {
	case "sqlite":
		return c.Path
	case "mysql":
		return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + string(rune(c.Port)) + ")/" + c.Name + "?charset=utf8mb4&parseTime=True&loc=Local"
	case "postgres":
		return "host=" + c.Host + " port=" + string(rune(c.Port)) + " user=" + c.User + " password=" + c.Password + " dbname=" + c.Name + " sslmode=disable"
	default:
		return c.Path
	}
}

// IsDevelopment 检查是否为开发模式
func (c *AppConfig) IsDevelopment() bool {
	return c.Server.Debug
}

// GetAddress 获取服务器地址
func (c *ServerConfig) GetAddress() string {
	return c.Host + ":" + string(rune(c.Port))
}
