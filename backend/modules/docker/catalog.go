// Package docker 应用商店目录服务
// 加载 docker-apps.yaml，提供应用浏览、搜索、分类查询
package docker

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// ==================== YAML 原始结构 ====================

// AppStoreRaw YAML 顶层结构
type AppStoreRaw struct {
	Apps    []AppRaw `yaml:"apps"`
	Version string   `yaml:"version"`
}

// AppRaw 单个应用的 YAML 原始结构
type AppRaw struct {
	ID            string            `yaml:"id"`
	Name          string            `yaml:"name"`
	Platform      string            `yaml:"platform"`
	Category      string            `yaml:"category"`
	Author        string            `yaml:"author,omitempty"`
	License       string            `yaml:"license,omitempty"`
	Homepage      string            `yaml:"homepage"`
	Repository    string            `yaml:"repository"`
	Icon          string            `yaml:"icon"`
	Version       string            `yaml:"version"`
	Architectures []string          `yaml:"architectures"`
	Tags          []string          `yaml:"tags"`
	Title         map[string]string `yaml:"title"`
	Description   map[string]string `yaml:"description"`
	Form          []FormFieldRaw    `yaml:"form"`
	Compose       yaml.Node         `yaml:"compose"` // 保持原始 YAML 结构
}

// FormFieldRaw 表单字段原始结构
type FormFieldRaw struct {
	Key      string            `yaml:"key"`
	Label    map[string]string `yaml:"label"`
	Default  interface{}       `yaml:"default,omitempty"`
	Required bool              `yaml:"required,omitempty"`
	Type     string            `yaml:"type"`
	EnvKey   string            `yaml:"env_key,omitempty"`
}

// ==================== API 响应结构 ====================

// StoreApp 商店应用（API 响应）
type StoreApp struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	TitleI18n     LocaleText  `json:"title_i18n"`
	DescI18n      LocaleText  `json:"description_i18n"`
	Category      string      `json:"category"`
	Icon          string      `json:"icon"`
	Version       string      `json:"version"`
	Author        string      `json:"author,omitempty"`
	License       string      `json:"license,omitempty"`
	Homepage      string      `json:"homepage"`
	Repository    string      `json:"repository"`
	Tags          []string    `json:"tags"`
	Architectures []string    `json:"architectures"`
	Form          []FormField `json:"form"`
	Compose       string      `json:"compose"` // YAML 字符串
}

// LocaleText 多语言文本
type LocaleText struct {
	En string `json:"en"`
	Zh string `json:"zh"`
}

// FormField 表单字段（API 响应）
type FormField struct {
	Key      string      `json:"key"`
	Label    LocaleText  `json:"label"`
	Default  interface{} `json:"default,omitempty"`
	Required bool        `json:"required"`
	Type     string      `json:"type"`
	EnvKey   string      `json:"env_key,omitempty"`
}

// StoreCategory 商店分类
type StoreCategory struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ==================== 目录服务 ====================

// CatalogService 应用目录服务
type CatalogService struct {
	apps       []StoreApp
	appsMap    map[string]*StoreApp
	categories []StoreCategory
	logger     *zap.Logger
}

// NewCatalogService 创建目录服务
func NewCatalogService(yamlPath string, logger *zap.Logger) (*CatalogService, error) {
	cs := &CatalogService{
		appsMap: make(map[string]*StoreApp),
		logger:  logger,
	}

	if err := cs.loadFromFile(yamlPath); err != nil {
		return nil, fmt.Errorf("failed to load app catalog: %w", err)
	}

	logger.Info("App catalog loaded",
		zap.Int("apps", len(cs.apps)),
		zap.Int("categories", len(cs.categories)),
	)

	return cs, nil
}

// loadFromFile 从 YAML 文件加载应用数据
func (cs *CatalogService) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var raw AppStoreRaw
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	// 转换为 API 结构
	cs.apps = make([]StoreApp, 0, len(raw.Apps))
	catCount := make(map[string]int)

	for _, r := range raw.Apps {
		if r.ID == "" {
			continue
		}

		app := StoreApp{
			ID:            r.ID,
			Name:          r.Name,
			Title:         pickLocale(r.Title, "zh"),
			Description:   pickLocale(r.Description, "zh"),
			TitleI18n:     LocaleText{En: r.Title["en"], Zh: r.Title["zh"]},
			DescI18n:      LocaleText{En: r.Description["en"], Zh: r.Description["zh"]},
			Category:      r.Category,
			Icon:          r.Icon,
			Version:       r.Version,
			Author:        r.Author,
			License:       r.License,
			Homepage:      r.Homepage,
			Repository:    r.Repository,
			Tags:          r.Tags,
			Architectures: r.Architectures,
			Form:          convertFormFields(r.Form),
		}

		// 将 compose 节点序列化为 YAML 字符串
		if r.Compose.Kind != 0 {
			composeBytes, err := yaml.Marshal(&r.Compose)
			if err == nil {
				app.Compose = string(composeBytes)
			}
		}

		cs.apps = append(cs.apps, app)
		cs.appsMap[r.ID] = &cs.apps[len(cs.apps)-1]
		catCount[r.Category]++
	}

	// 构建分类列表
	cs.categories = make([]StoreCategory, 0, len(catCount))
	for cat, count := range catCount {
		cs.categories = append(cs.categories, StoreCategory{
			ID:    cat,
			Name:  categoryName(cat),
			Count: count,
		})
	}

	return nil
}

// GetApps 获取应用列表（支持分类和搜索筛选）
func (cs *CatalogService) GetApps(category, search string) []StoreApp {
	if category == "" && search == "" {
		return cs.apps
	}

	searchLower := strings.ToLower(search)
	result := make([]StoreApp, 0)

	for _, app := range cs.apps {
		// 分类筛选
		if category != "" && app.Category != category {
			continue
		}

		// 搜索筛选
		if search != "" {
			match := strings.Contains(strings.ToLower(app.Name), searchLower) ||
				strings.Contains(strings.ToLower(app.Title), searchLower) ||
				strings.Contains(strings.ToLower(app.Description), searchLower) ||
				strings.Contains(strings.ToLower(app.ID), searchLower) ||
				strings.Contains(strings.ToLower(app.TitleI18n.En), searchLower)
			if !match {
				// 检查标签
				for _, tag := range app.Tags {
					if strings.Contains(strings.ToLower(tag), searchLower) {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}

		result = append(result, app)
	}

	return result
}

// GetApp 获取单个应用详情
func (cs *CatalogService) GetApp(id string) *StoreApp {
	return cs.appsMap[id]
}

// GetCategories 获取分类列表
func (cs *CatalogService) GetCategories() []StoreCategory {
	return cs.categories
}

// AppCount 获取应用数量
func (cs *CatalogService) AppCount() int {
	return len(cs.apps)
}

// ==================== 辅助函数 ====================

func pickLocale(m map[string]string, preferred string) string {
	if v, ok := m[preferred]; ok && v != "" {
		return v
	}
	if v, ok := m["en"]; ok && v != "" {
		return v
	}
	for _, v := range m {
		return v
	}
	return ""
}

func convertFormFields(raw []FormFieldRaw) []FormField {
	fields := make([]FormField, 0, len(raw))
	for _, r := range raw {
		fields = append(fields, FormField{
			Key:      r.Key,
			Label:    LocaleText{En: r.Label["en"], Zh: r.Label["zh"]},
			Default:  r.Default,
			Required: r.Required,
			Type:     r.Type,
			EnvKey:   r.EnvKey,
		})
	}
	return fields
}

func categoryName(id string) string {
	names := map[string]string{
		"database":      "数据库",
		"development":   "开发工具",
		"ai":            "AI / 机器学习",
		"storage":       "存储",
		"media":         "媒体",
		"monitoring":    "监控",
		"productivity":  "生产力",
		"utilities":     "实用工具",
		"security":      "安全",
		"network":       "网络",
		"communication": "通讯协作",
		"gaming":        "游戏",
		"automation":    "自动化",
		"finance":       "财务",
		"other":         "其他",
	}
	if name, ok := names[id]; ok {
		return name
	}
	return id
}
