// Package windows HTTP 处理器
package windows

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器实例
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	win := r.Group("/windows")
	{
		win.GET("/info", h.GetWineInfo)

		win.GET("/prefixes", h.GetPrefixes)
		win.POST("/prefixes", h.CreatePrefix)
		win.GET("/prefixes/:id", h.GetPrefix)
		win.DELETE("/prefixes/:id", h.DeletePrefix)
		win.POST("/prefixes/:id/winetricks", h.RunWinetricks)

		win.GET("/apps", h.GetApps)
		win.POST("/apps", h.AddApp)
		win.GET("/apps/:id", h.GetApp)
		win.PUT("/apps/:id", h.UpdateApp)
		win.DELETE("/apps/:id", h.DeleteApp)
		win.POST("/apps/install", h.InstallApp)

		win.GET("/sessions", h.GetSessions)
		win.POST("/sessions/launch", h.LaunchApp)
		win.POST("/sessions/run", h.RunExe)
		win.DELETE("/sessions/:id", h.StopSession)

		win.GET("/store", h.GetStoreApps)
	}
}

// GetWineInfo 获取 Wine 信息
func (h *Handler) GetWineInfo(c *gin.Context) {
	info := h.service.GetWineInfo()
	c.JSON(http.StatusOK, info)
}

// GetPrefixes 获取前缀列表
func (h *Handler) GetPrefixes(c *gin.Context) {
	prefixes := h.service.GetPrefixes()
	c.JSON(http.StatusOK, prefixes)
}

// GetPrefix 获取前缀
func (h *Handler) GetPrefix(c *gin.Context) {
	id := c.Param("id")
	prefix, err := h.service.GetPrefix(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prefix)
}

// CreatePrefix 创建前缀
func (h *Handler) CreatePrefix(c *gin.Context) {
	var req CreatePrefixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	prefix, err := h.service.CreatePrefix(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prefix)
}

// DeletePrefix 删除前缀
func (h *Handler) DeletePrefix(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeletePrefix(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "prefix deleted"})
}

// RunWinetricks 运行 Winetricks
func (h *Handler) RunWinetricks(c *gin.Context) {
	id := c.Param("id")
	var req WinetricksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.PrefixID = id

	if err := h.service.RunWinetricks(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "winetricks completed"})
}

// GetApps 获取应用列表
func (h *Handler) GetApps(c *gin.Context) {
	prefixID := c.Query("prefix_id")
	apps := h.service.GetApps(prefixID)
	c.JSON(http.StatusOK, apps)
}

// GetApp 获取应用
func (h *Handler) GetApp(c *gin.Context) {
	id := c.Param("id")
	app, err := h.service.GetApp(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

// AddApp 添加应用
func (h *Handler) AddApp(c *gin.Context) {
	var req AddAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app, err := h.service.AddApp(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

// UpdateApp 更新应用
func (h *Handler) UpdateApp(c *gin.Context) {
	id := c.Param("id")
	var req UpdateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app, err := h.service.UpdateApp(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

// DeleteApp 删除应用
func (h *Handler) DeleteApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteApp(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app deleted"})
}

// InstallApp 安装应用
func (h *Handler) InstallApp(c *gin.Context) {
	var req InstallAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app, err := h.service.InstallApp(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

// GetSessions 获取会话列表
func (h *Handler) GetSessions(c *gin.Context) {
	sessions := h.service.GetSessions()
	c.JSON(http.StatusOK, sessions)
}

// LaunchApp 启动应用
func (h *Handler) LaunchApp(c *gin.Context) {
	var req LaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.service.LaunchApp(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// RunExe 运行 EXE
func (h *Handler) RunExe(c *gin.Context) {
	var req RunExeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.service.RunExe(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// StopSession 停止会话
func (h *Handler) StopSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.StopSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "session stopped"})
}

// GetStoreApps 获取商店应用
func (h *Handler) GetStoreApps(c *gin.Context) {
	apps := h.service.GetStoreApps()
	c.JSON(http.StatusOK, apps)
}
