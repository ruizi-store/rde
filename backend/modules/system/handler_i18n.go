// Package system i18n 设置处理器
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/i18n"
)

// I18nSettingsRequest i18n 设置请求
type I18nSettingsRequest struct {
	Region      string            `json:"region" binding:"omitempty"`
	Mirrors     map[string]string `json:"mirrors,omitempty"`
	LyricSource string            `json:"lyric_source,omitempty"`
}

// I18nSettingsResponse i18n 设置响应
type I18nSettingsResponse struct {
	Region         string            `json:"region"`
	DetectedRegion string            `json:"detected_region"`
	Mirrors        map[string]string `json:"mirrors"`
	LyricSource    string            `json:"lyric_source"`
}

// MirrorOptionsResponse 镜像选项响应
type MirrorOptionsResponse struct {
	Languages    []i18n.LanguageOption           `json:"languages"`
	Regions      []i18n.RegionOption             `json:"regions"`
	Mirrors      map[string]map[string][]MirrorOption `json:"mirrors"`
	LyricSources map[string][]i18n.LyricSource   `json:"lyric_sources"`
	Services     []MirrorServiceInfo             `json:"services"`
}

// MirrorOption 镜像选项
type MirrorOption struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Priority int    `json:"priority,omitempty"`
}

// MirrorServiceInfo 镜像服务信息
type MirrorServiceInfo struct {
	ID   string            `json:"id"`
	Name map[string]string `json:"name"`
}

// RegionSwitchPreview 区域切换预览
type RegionSwitchPreviewRequest struct {
	FromRegion string `json:"from_region" binding:"required"`
	ToRegion   string `json:"to_region" binding:"required"`
}

// RegionSwitchPreviewResponse 区域切换预览响应
type RegionSwitchPreviewResponse struct {
	Items []RegionSwitchItem `json:"items"`
}

// RegionSwitchItem 区域切换项
type RegionSwitchItem struct {
	Service     string            `json:"service"`
	ServiceName map[string]string `json:"service_name"`
	CurrentURL  string            `json:"current_url"`
	NewURL      string            `json:"new_url"`
	Enabled     bool              `json:"enabled"`
}

// GetI18nSettings 获取 i18n 设置
// @Summary 获取国际化设置
// @Tags system
// @Produce json
// @Success 200 {object} I18nSettingsResponse
// @Router /api/v1/system/i18n [get]
func (h *Handler) GetI18nSettings(c *gin.Context) {
	settings, err := h.service.GetI18nSettings()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, settings)
}

// UpdateI18nSettings 更新 i18n 设置
// @Summary 更新国际化设置
// @Tags system
// @Accept json
// @Produce json
// @Param body body I18nSettingsRequest true "设置"
// @Success 200 {object} I18nSettingsResponse
// @Router /api/v1/system/i18n [put]
func (h *Handler) UpdateI18nSettings(c *gin.Context) {
	var req I18nSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	// 验证区域
	if req.Region != "" && !i18n.IsValidRegion(req.Region) {
		fail(c, 400, "invalid region code")
		return
	}

	settings, err := h.service.UpdateI18nSettings(req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, settings)
}

// GetI18nOptions 获取 i18n 选项（语言、区域、镜像列表）
// @Summary 获取国际化选项
// @Tags system
// @Produce json
// @Success 200 {object} MirrorOptionsResponse
// @Router /api/v1/system/i18n/options [get]
func (h *Handler) GetI18nOptions(c *gin.Context) {
	config := i18n.GetMirrorsConfig()

	// 构建镜像选项
	mirrors := make(map[string]map[string][]MirrorOption)
	for regionCode, regionMirrors := range config.Regions {
		mirrors[regionCode] = map[string][]MirrorOption{
			"apt":     convertMirrorEntries(regionMirrors.APT),
			"docker":  convertMirrorEntries(regionMirrors.Docker),
			"npm":     {convertMirrorEntry(regionMirrors.NPM)},
			"pip":     {{Name: "PyPI", URL: regionMirrors.PIP.URL}},
			"flatpak": {convertMirrorEntry(regionMirrors.Flatpak)},
		}
	}

	// 构建服务列表
	services := make([]MirrorServiceInfo, 0, len(i18n.MirrorServices))
	for _, s := range i18n.MirrorServices {
		services = append(services, MirrorServiceInfo{
			ID:   s.ID,
			Name: s.Name,
		})
	}

	response := MirrorOptionsResponse{
		Languages:    i18n.SupportedLanguages,
		Regions:      i18n.SupportedRegions,
		Mirrors:      mirrors,
		LyricSources: config.LyricSources,
		Services:     services,
	}

	ok(c, response)
}

// PreviewRegionSwitch 预览区域切换影响
// @Summary 预览区域切换
// @Tags system
// @Accept json
// @Produce json
// @Param body body RegionSwitchPreviewRequest true "切换请求"
// @Success 200 {object} RegionSwitchPreviewResponse
// @Router /api/v1/system/i18n/preview-switch [post]
func (h *Handler) PreviewRegionSwitch(c *gin.Context) {
	var req RegionSwitchPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	if !i18n.IsValidRegion(req.FromRegion) || !i18n.IsValidRegion(req.ToRegion) {
		fail(c, 400, "invalid region code")
		return
	}

	items := h.service.PreviewRegionSwitch(req.FromRegion, req.ToRegion)
	ok(c, RegionSwitchPreviewResponse{Items: items})
}

// DetectRegion 检测用户区域
// @Summary 检测用户区域
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/v1/system/i18n/detect [get]
func (h *Handler) DetectRegion(c *gin.Context) {
	detectedRegion := i18n.DetectRegion()
	detectedLanguage := i18n.DetectLanguage()

	ok(c, gin.H{
		"region":             detectedRegion,
		"language":           detectedLanguage,
		"suggested_region":   detectedRegion,
		"suggested_language": detectedLanguage,
	})
}

// convertMirrorEntries 转换镜像条目列表
func convertMirrorEntries(entries []i18n.MirrorEntry) []MirrorOption {
	result := make([]MirrorOption, 0, len(entries))
	for _, e := range entries {
		result = append(result, MirrorOption{
			Name:     e.Name,
			URL:      e.URL,
			Priority: e.Priority,
		})
	}
	return result
}

// convertMirrorEntry 转换单个镜像条目
func convertMirrorEntry(entry i18n.MirrorEntry) MirrorOption {
	return MirrorOption{
		Name:     entry.Name,
		URL:      entry.URL,
		Priority: entry.Priority,
	}
}
