package model

import (
	"time"
)

// DDNSProvider DDNS 提供商类型
type DDNSProvider string

const (
	DDNSProviderCloudflare DDNSProvider = "cloudflare"
	DDNSProviderGoDaddy    DDNSProvider = "godaddy"
	DDNSProviderAliyun     DDNSProvider = "aliyun"
	DDNSProviderDNSPod     DDNSProvider = "dnspod"
	DDNSProviderDynDNS     DDNSProvider = "dyndns"
	DDNSProviderNoIP       DDNSProvider = "noip"
	DDNSProviderDuckDNS    DDNSProvider = "duckdns"
	DDNSProviderCustom     DDNSProvider = "custom"
)

// DDNSStatus DDNS 状态
type DDNSStatus string

const (
	DDNSStatusActive   DDNSStatus = "active"
	DDNSStatusInactive DDNSStatus = "inactive"
	DDNSStatusError    DDNSStatus = "error"
)

// DDNSConfig DDNS 配置
type DDNSConfig struct {
	ID             string       `json:"id" gorm:"primaryKey;size:36"`
	Name           string       `json:"name" gorm:"size:100;not null"`
	Provider       DDNSProvider `json:"provider" gorm:"size:30;not null"`
	Enabled        bool         `json:"enabled" gorm:"default:false"`
	Domain         string       `json:"domain" gorm:"size:200;not null"`    // 完整域名
	Subdomain      string       `json:"subdomain" gorm:"size:100"`          // 子域名
	Config         string       `json:"config" gorm:"type:text"`            // JSON 配置（API密钥等）
	UpdateInterval int          `json:"update_interval" gorm:"default:300"` // 更新间隔（秒）
	CurrentIP      string       `json:"current_ip" gorm:"size:50"`
	LastIP         string       `json:"last_ip" gorm:"size:50"`
	Status         DDNSStatus   `json:"status" gorm:"size:20;default:inactive"`
	LastUpdateAt   *time.Time   `json:"last_update_at"`
	LastCheckAt    *time.Time   `json:"last_check_at"`
	LastError      string       `json:"last_error" gorm:"size:500"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// TableName 表名
func (DDNSConfig) TableName() string {
	return "ddns_configs"
}

// DDNSLog DDNS 更新日志
type DDNSLog struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	ConfigID  string    `json:"config_id" gorm:"size:36;index"`
	OldIP     string    `json:"old_ip" gorm:"size:50"`
	NewIP     string    `json:"new_ip" gorm:"size:50"`
	Success   bool      `json:"success" gorm:"default:false"`
	Message   string    `json:"message" gorm:"size:500"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName 表名
func (DDNSLog) TableName() string {
	return "ddns_logs"
}

// ===================== 提供商配置结构 =====================

// CloudflareConfig Cloudflare 配置
type CloudflareConfig struct {
	APIToken string `json:"api_token"` // API Token 或 Global API Key
	ZoneID   string `json:"zone_id"`   // Zone ID
	Email    string `json:"email"`     // 可选，使用 Global API Key 时需要
	Proxied  bool   `json:"proxied"`   // 是否启用 Cloudflare 代理
}

// GoDaddyConfig GoDaddy 配置
type GoDaddyConfig struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

// AliyunConfig 阿里云 DNS 配置
type AliyunConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	RegionID        string `json:"region_id"`
}

// DNSPodConfig DNSPod 配置
type DNSPodConfig struct {
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
}

// DynDNSConfig DynDNS 配置
type DynDNSConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NoIPConfig No-IP 配置
type NoIPConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DuckDNSConfig DuckDNS 配置
type DuckDNSConfig struct {
	Token string `json:"token"`
}

// CustomDDNSConfig 自定义 DDNS 配置
type CustomDDNSConfig struct {
	UpdateURL string            `json:"update_url"` // 更新 URL，支持变量替换
	Method    string            `json:"method"`     // HTTP 方法
	Headers   map[string]string `json:"headers"`    // 自定义请求头
	Body      string            `json:"body"`       // 请求体模板
}

// ===================== 请求/响应结构 =====================

// CreateDDNSConfigRequest 创建 DDNS 配置请求
type CreateDDNSConfigRequest struct {
	Name           string       `json:"name" validate:"required"`
	Provider       DDNSProvider `json:"provider" validate:"required"`
	Domain         string       `json:"domain" validate:"required"`
	Subdomain      string       `json:"subdomain"`
	Config         interface{}  `json:"config" validate:"required"`
	UpdateInterval int          `json:"update_interval"`
	Enabled        bool         `json:"enabled"`
}

// UpdateDDNSConfigRequest 更新 DDNS 配置请求
type UpdateDDNSConfigRequest struct {
	Name           string      `json:"name"`
	Domain         string      `json:"domain"`
	Subdomain      string      `json:"subdomain"`
	Config         interface{} `json:"config"`
	UpdateInterval *int        `json:"update_interval"`
	Enabled        *bool       `json:"enabled"`
}

// DDNSStatsResponse DDNS 统计响应
type DDNSStatsResponse struct {
	TotalConfigs   int64  `json:"total_configs"`
	ActiveConfigs  int64  `json:"active_configs"`
	ErrorConfigs   int64  `json:"error_configs"`
	CurrentIP      string `json:"current_ip"`
	LastUpdateTime string `json:"last_update_time"`
}

// GoDaddyModel 旧版 GoDaddy 模型（保留兼容）
type GoDaddyModel struct {
	Type    uint   `json:"type"`
	ApiHost string `json:"api_host"`
	Key     string `json:"key"`
	Secret  string `json:"secret"`
	Host    string `json:"host"`
}

