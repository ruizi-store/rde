// Package system i18n 服务
package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/ruizi-store/rde/backend/core/i18n"
)

// i18n 设置文件名
const i18nSettingsFile = "i18n.json"

// i18nSettings 缓存
var (
	i18nCache     *I18nSettingsData
	i18nCacheMux  sync.RWMutex
)

// I18nSettingsData i18n 设置数据（language 由前端管理，不在后端存储）
type I18nSettingsData struct {
	Region      string            `json:"region"`
	Mirrors     map[string]string `json:"mirrors"`      // service -> "follow" | "cn" | "intl" | "custom"
	CustomURLs  map[string]string `json:"custom_urls"`  // service -> custom URL
	LyricSource string            `json:"lyric_source"` // "follow" | source id
}

// GetI18nSettings 获取 i18n 设置
func (s *Service) GetI18nSettings() (*I18nSettingsResponse, error) {
	data, err := s.loadI18nSettings()
	if err != nil {
		return nil, err
	}

	return &I18nSettingsResponse{
		Region:         data.Region,
		DetectedRegion: i18n.DetectRegion(),
		Mirrors:        data.Mirrors,
		LyricSource:    data.LyricSource,
	}, nil
}

// UpdateI18nSettings 更新 i18n 设置
func (s *Service) UpdateI18nSettings(req I18nSettingsRequest) (*I18nSettingsResponse, error) {
	data, err := s.loadI18nSettings()
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.Region != "" {
		data.Region = req.Region
	}
	if req.Mirrors != nil {
		for k, v := range req.Mirrors {
			data.Mirrors[k] = v
		}
	}
	if req.LyricSource != "" {
		data.LyricSource = req.LyricSource
	}

	// 保存
	if err := s.saveI18nSettings(data); err != nil {
		return nil, err
	}

	// 更新全局区域覆盖
	if data.Region != "" {
		i18n.SetRegionOverride(data.Region)
	}

	// 发布事件
	s.publishEvent("system.i18n.changed", map[string]interface{}{
		"region": data.Region,
	})

	return &I18nSettingsResponse{
		Region:         data.Region,
		DetectedRegion: i18n.DetectRegion(),
		Mirrors:        data.Mirrors,
		LyricSource:    data.LyricSource,
	}, nil
}

// PreviewRegionSwitch 预览区域切换
func (s *Service) PreviewRegionSwitch(fromRegion, toRegion string) []RegionSwitchItem {
	data, _ := s.loadI18nSettings()
	items := make([]RegionSwitchItem, 0)

	for _, svc := range i18n.MirrorServices {
		currentSetting := i18n.MirrorFollow
		if data.Mirrors != nil {
			if v, ok := data.Mirrors[svc.ID]; ok {
				currentSetting = v
			}
		}

		// 只有跟随区域的服务才会受影响
		enabled := currentSetting == i18n.MirrorFollow

		currentURL := i18n.GetMirrorURL(svc.ID, currentSetting, fromRegion, "")
		newURL := i18n.GetMirrorURL(svc.ID, i18n.MirrorFollow, toRegion, "")

		items = append(items, RegionSwitchItem{
			Service:     svc.ID,
			ServiceName: svc.Name,
			CurrentURL:  currentURL,
			NewURL:      newURL,
			Enabled:     enabled,
		})
	}

	return items
}

// GetMirrorURL 获取指定服务的当前镜像 URL
func (s *Service) GetMirrorURL(service string) string {
	data, _ := s.loadI18nSettings()
	
	setting := i18n.MirrorFollow
	if data.Mirrors != nil {
		if v, ok := data.Mirrors[service]; ok {
			setting = v
		}
	}
	
	customURL := ""
	if data.CustomURLs != nil {
		customURL = data.CustomURLs[service]
	}
	
	return i18n.GetMirrorURL(service, setting, data.Region, customURL)
}

// loadI18nSettings 加载 i18n 设置
func (s *Service) loadI18nSettings() (*I18nSettingsData, error) {
	i18nCacheMux.RLock()
	if i18nCache != nil {
		defer i18nCacheMux.RUnlock()
		return i18nCache, nil
	}
	i18nCacheMux.RUnlock()

	i18nCacheMux.Lock()
	defer i18nCacheMux.Unlock()

	// 双重检查
	if i18nCache != nil {
		return i18nCache, nil
	}

	settingsPath := filepath.Join(s.dataPath, i18nSettingsFile)
	
	data := &I18nSettingsData{
		Region:      i18n.DetectRegion(),
		Mirrors:     make(map[string]string),
		CustomURLs:  make(map[string]string),
		LyricSource: i18n.MirrorFollow,
	}

	// 初始化所有镜像服务为跟随模式
	for _, svc := range i18n.MirrorServices {
		data.Mirrors[svc.ID] = i18n.MirrorFollow
	}

	// 尝试从文件读取
	if fileData, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(fileData, data); err != nil {
			s.logger.Warn("Failed to parse i18n settings, using defaults")
		}
	}

	// 设置全局区域覆盖，让 i18n.DetectRegion() 返回用户保存的区域
	if data.Region != "" {
		i18n.SetRegionOverride(data.Region)
	}

	i18nCache = data
	return data, nil
}

// saveI18nSettings 保存 i18n 设置
func (s *Service) saveI18nSettings(data *I18nSettingsData) error {
	i18nCacheMux.Lock()
	defer i18nCacheMux.Unlock()

	settingsPath := filepath.Join(s.dataPath, i18nSettingsFile)
	
	// 确保目录存在
	if err := os.MkdirAll(s.dataPath, 0755); err != nil {
		return err
	}

	fileData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(settingsPath, fileData, 0644); err != nil {
		return err
	}

	i18nCache = data
	return nil
}

// InvalidateI18nCache 清除 i18n 缓存
func InvalidateI18nCache() {
	i18nCacheMux.Lock()
	defer i18nCacheMux.Unlock()
	i18nCache = nil
}
