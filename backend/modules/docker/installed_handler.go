// Package docker 已安装应用 HTTP 处理器
package docker

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// InstalledHandler 已安装应用 HTTP 处理器
type InstalledHandler struct {
	installed *InstalledService
	logger    *zap.Logger
}

// NewInstalledHandler 创建处理器
func NewInstalledHandler(installed *InstalledService, logger *zap.Logger) *InstalledHandler {
	return &InstalledHandler{
		installed: installed,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由
func (h *InstalledHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("", h.List)
	r.POST("", h.Install)
	r.POST("/async", h.InstallAsync)
	r.GET("/tasks/:taskId", h.GetTask)
	r.DELETE("/:name", h.Uninstall)
	r.POST("/:name/start", h.Start)
	r.POST("/:name/stop", h.Stop)
	r.POST("/:name/restart", h.Restart)
	r.GET("/:name/logs", h.GetLogs)
}

// List 获取已安装应用列表
// GET /docker/apps
func (h *InstalledHandler) List(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}
	apps := h.installed.GetAll()
	c.JSON(http.StatusOK, gin.H{"data": apps})
}

// Install 安装应用
// POST /docker/apps { app_id, config }
func (h *InstalledHandler) Install(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	var req struct {
		AppID  string                 `json:"app_id" binding:"required"`
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Config == nil {
		req.Config = make(map[string]interface{})
	}

	app, output, err := h.installed.Install(req.AppID, req.Config)
	if err != nil {
		h.logger.Error("App install failed",
			zap.String("app_id", req.AppID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"output": output,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data":   app,
		"output": output,
	})
}

// InstallAsync 异步安装应用
// POST /docker/apps/async { app_id, config }
func (h *InstalledHandler) InstallAsync(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	var req struct {
		AppID  string                 `json:"app_id" binding:"required"`
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Config == nil {
		req.Config = make(map[string]interface{})
	}

	task, err := h.installed.InstallAsync(req.AppID, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"data": task})
}

// GetTask 获取安装任务状态
// GET /docker/apps/tasks/:taskId
func (h *InstalledHandler) GetTask(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	taskID := c.Param("taskId")
	task := h.installed.GetTask(taskID)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": task})
}

// Uninstall 卸载应用
// DELETE /docker/apps/:name
func (h *InstalledHandler) Uninstall(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	name := c.Param("name")
	if err := h.installed.Uninstall(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "App uninstalled"})
}

// Start 启动应用
// POST /docker/apps/:name/start
func (h *InstalledHandler) Start(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	name := c.Param("name")
	if err := h.installed.Start(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "App started"})
}

// Stop 停止应用
// POST /docker/apps/:name/stop
func (h *InstalledHandler) Stop(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	name := c.Param("name")
	if err := h.installed.Stop(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "App stopped"})
}

// Restart 重启应用
// POST /docker/apps/:name/restart
func (h *InstalledHandler) Restart(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	name := c.Param("name")
	if err := h.installed.Restart(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "App restarted"})
}

// GetLogs 获取应用日志
// GET /docker/apps/:name/logs?tail=100
func (h *InstalledHandler) GetLogs(c *gin.Context) {
	if h.installed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Installed service not available"})
		return
	}

	name := c.Param("name")
	tail := 100
	if t := c.Query("tail"); t != "" {
		fmt.Sscanf(t, "%d", &tail)
	}

	logs, err := h.installed.GetLogs(name, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}
