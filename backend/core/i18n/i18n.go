// Package i18n 提供国际化和区域配置服务
package i18n

import (
	"os"
	"strings"
	"sync"
	"time"
)

// 支持的语言
const (
	LangZhCN = "zh-CN" // 简体中文
	LangEnUS = "en-US" // English
)

// 支持的区域
const (
	RegionCN   = "cn"   // 中国大陆
	RegionIntl = "intl" // 国际
)

// 镜像源跟随模式
const (
	MirrorFollow = "follow" // 跟随区域设置
	MirrorCN     = "cn"     // 强制中国源
	MirrorIntl   = "intl"   // 强制国际源
	MirrorCustom = "custom" // 自定义
)

// 支持语言列表
var SupportedLanguages = []LanguageOption{
	{Code: LangZhCN, Name: "简体中文", NativeName: "简体中文"},
	{Code: LangEnUS, Name: "English", NativeName: "English"},
}

// 支持区域列表
var SupportedRegions = []RegionOption{
	{Code: RegionCN, Name: map[string]string{"zh-CN": "中国大陆", "en-US": "China Mainland"}, Description: map[string]string{"zh-CN": "使用国内镜像源，速度更快", "en-US": "Use China mirrors for faster speed"}},
	{Code: RegionIntl, Name: map[string]string{"zh-CN": "国际", "en-US": "International"}, Description: map[string]string{"zh-CN": "使用官方源", "en-US": "Use official sources"}},
}

// LanguageOption 语言选项
type LanguageOption struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
}

// RegionOption 区域选项
type RegionOption struct {
	Code        string            `json:"code"`
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
}

// I18nSettings i18n 设置
type I18nSettings struct {
	Language       string                   `json:"language"`
	Region         string                   `json:"region"`
	DetectedRegion string                   `json:"detected_region"`
	Mirrors        map[string]string        `json:"mirrors"`
	LyricSource    string                   `json:"lyric_source"`
}

// MirrorSwitchItem 镜像切换确认项
type MirrorSwitchItem struct {
	Service     string `json:"service"`      // apt, docker, npm, pip, flatpak
	ServiceName map[string]string `json:"service_name"` // 显示名称
	CurrentURL  string `json:"current_url"`  // 当前 URL
	NewURL      string `json:"new_url"`      // 新 URL
	Enabled     bool   `json:"enabled"`      // 是否启用切换
}

// regionOverride 用户手动设置的区域覆盖（优先于自动检测）
var (
	regionOverride    string
	regionOverrideMux sync.RWMutex
)

// SetRegionOverride 设置区域覆盖（由启动时从 i18n.json 读取用户设置后调用）
func SetRegionOverride(region string) {
	regionOverrideMux.Lock()
	defer regionOverrideMux.Unlock()
	regionOverride = region
}

// DetectRegion 自动检测用户区域（优先使用用户设置的覆盖值）
func DetectRegion() string {
	// 0. 优先使用用户手动设置的区域
	regionOverrideMux.RLock()
	override := regionOverride
	regionOverrideMux.RUnlock()
	if override != "" {
		return override
	}

	// 1. 检查环境变量 LANG
	lang := os.Getenv("LANG")
	if strings.HasPrefix(lang, "zh_CN") {
		return RegionCN
	}

	// 2. 检查 LC_ALL
	lcAll := os.Getenv("LC_ALL")
	if strings.HasPrefix(lcAll, "zh_CN") {
		return RegionCN
	}

	// 3. 检查时区
	loc := time.Now().Location()
	tz := loc.String()
	if tz == "Asia/Shanghai" || tz == "Asia/Chongqing" || tz == "Asia/Harbin" || 
	   tz == "Asia/Urumqi" || tz == "PRC" || tz == "Local" {
		// Local 时区在中国系统上通常指向上海
		// 进一步检查时区偏移
		_, offset := time.Now().Zone()
		if offset == 8*3600 { // UTC+8
			// UTC+8 且未明确设置英文 → 使用国内镜像
			// 覆盖中国、新加坡等东八区，这些地区用国内镜像更快
			if !strings.HasPrefix(lang, "en") {
				return RegionCN
			}
		}
	}

	// 4. 检查是否有中文相关的环境变量
	if strings.Contains(lang, "zh") || strings.Contains(lang, "CN") {
		return RegionCN
	}

	return RegionIntl
}

// DetectLanguage 自动检测用户语言
func DetectLanguage() string {
	// 检查 LANG 环境变量
	lang := os.Getenv("LANG")
	if strings.HasPrefix(lang, "zh") {
		return LangZhCN
	}
	
	// 检查 LANGUAGE 环境变量
	language := os.Getenv("LANGUAGE")
	if strings.HasPrefix(language, "zh") {
		return LangZhCN
	}

	return LangEnUS
}

// GetDefaultRegionForLanguage 根据语言获取推荐区域
func GetDefaultRegionForLanguage(lang string) string {
	switch lang {
	case LangZhCN:
		return RegionCN
	default:
		return RegionIntl
	}
}

// GetDefaultLanguageForRegion 根据区域获取推荐语言
func GetDefaultLanguageForRegion(region string) string {
	switch region {
	case RegionCN:
		return LangZhCN
	default:
		return LangEnUS
	}
}

// IsValidLanguage 检查语言代码是否有效
func IsValidLanguage(lang string) bool {
	for _, l := range SupportedLanguages {
		if l.Code == lang {
			return true
		}
	}
	return false
}

// IsValidRegion 检查区域代码是否有效
func IsValidRegion(region string) bool {
	for _, r := range SupportedRegions {
		if r.Code == region {
			return true
		}
	}
	return false
}
