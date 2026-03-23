// Package translate HTTP 处理器
package translate

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	translate := r.Group("/translate")
	{
		// 翻译
		translate.POST("/text", h.TranslateText)

		// 语言检测
		translate.POST("/detect", h.DetectLanguage)

		// 语言列表
		translate.GET("/languages", h.GetLanguages)

		// 服务状态
		translate.GET("/status", h.GetStatus)

		// 配置
		translate.GET("/config", h.GetConfig)
	}
}

// TranslateText 翻译文本
// @Summary 翻译文本
// @Tags translate
// @Accept json
// @Produce json
// @Param request body TranslateRequest true "翻译请求"
// @Success 200 {object} TranslateResponse
// @Router /translate/text [post]
func (h *Handler) TranslateText(c *gin.Context) {
	var req TranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 验证文本不为空
	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "翻译文本不能为空"})
		return
	}

	// 验证目标语言
	if req.Target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标语言不能为空"})
		return
	}

	resp, err := h.service.Translate(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "翻译失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DetectLanguage 检测语言
// @Summary 检测文本语言
// @Tags translate
// @Accept json
// @Produce json
// @Param request body DetectRequest true "检测请求"
// @Success 200 {object} DetectResponse
// @Router /translate/detect [post]
func (h *Handler) DetectLanguage(c *gin.Context) {
	var req DetectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "检测文本不能为空"})
		return
	}

	resp, err := h.service.DetectLanguage(req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "语言检测失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetLanguages 获取支持的语言列表
// @Summary 获取支持的语言列表
// @Tags translate
// @Produce json
// @Success 200 {array} Language
// @Router /translate/languages [get]
func (h *Handler) GetLanguages(c *gin.Context) {
	languages, err := h.service.GetLanguages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取语言列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, languages)
}

// GetStatus 获取服务状态
// @Summary 获取翻译服务状态
// @Tags translate
// @Produce json
// @Success 200 {object} ServiceStatus
// @Router /translate/status [get]
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.service.CheckStatus()
	c.JSON(http.StatusOK, status)
}

// GetConfig 获取配置
// @Summary 获取翻译配置
// @Tags translate
// @Produce json
// @Success 200 {object} TranslateConfig
// @Router /translate/config [get]
func (h *Handler) GetConfig(c *gin.Context) {
	// 从请求头或查询参数获取系统语言
	systemLang := c.Query("lang")
	if systemLang == "" {
		systemLang = c.GetHeader("Accept-Language")
	}
	if systemLang == "" {
		systemLang = "en"
	}

	config := h.service.GetConfig(systemLang)
	c.JSON(http.StatusOK, config)
}
