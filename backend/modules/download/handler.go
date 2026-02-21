// Package download HTTP 处理器
package download

import (
	"fmt"
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
	download := r.Group("/download")
	{
		// WebSocket 实时推送
		download.GET("/ws", h.WebSocket)

		// 任务管理
		download.GET("/tasks", h.GetTasks)
		download.GET("/tasks/:gid", h.GetTask)
		download.POST("/tasks/uri", h.AddURI)
		download.POST("/tasks/torrent", h.AddTorrent)
		download.POST("/tasks/metalink", h.AddMetalink)

		download.POST("/tasks/:gid/pause", h.Pause)
		download.POST("/tasks/:gid/resume", h.Resume)
		download.DELETE("/tasks/:gid", h.Remove)
		download.DELETE("/tasks/:gid/result", h.RemoveResult)

		download.POST("/pause-all", h.PauseAll)
		download.POST("/resume-all", h.ResumeAll)
		download.DELETE("/results", h.PurgeResults)

		download.GET("/stats", h.GetStats)
		download.PUT("/options", h.UpdateOptions)

		download.POST("/start", h.Start)
		download.POST("/stop", h.Stop)

		// 历史记录
		download.GET("/history", h.GetHistory)
		download.GET("/history/search", h.SearchHistory)
		download.DELETE("/history", h.ClearHistory)
		download.DELETE("/history/:id", h.DeleteHistoryItem)

		// 用户设置
		download.GET("/settings", h.GetSettings)
		download.PUT("/settings", h.UpdateSettings)

		// 统计
		download.GET("/statistics", h.GetStatistics)
	}
}

// GetTasks 获取任务列表
func (h *Handler) GetTasks(c *gin.Context) {
	tasks, err := h.service.GetTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// GetTask 获取单个任务
func (h *Handler) GetTask(c *gin.Context) {
	gid := c.Param("gid")
	task, err := h.service.GetTask(gid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// AddURI 添加 URI 下载
func (h *Handler) AddURI(c *gin.Context) {
	var req AddURIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gid, err := h.service.AddURI(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"gid": gid})
}

// AddTorrent 添加种子下载
func (h *Handler) AddTorrent(c *gin.Context) {
	var req AddTorrentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gid, err := h.service.AddTorrent(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"gid": gid})
}

// AddMetalink 添加 Metalink 下载
func (h *Handler) AddMetalink(c *gin.Context) {
	var req AddMetalinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gids, err := h.service.AddMetalink(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"gids": gids})
}

// Pause 暂停任务
func (h *Handler) Pause(c *gin.Context) {
	gid := c.Param("gid")
	if err := h.service.Pause(gid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task paused"})
}

// Resume 恢复任务
func (h *Handler) Resume(c *gin.Context) {
	gid := c.Param("gid")
	if err := h.service.Resume(gid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task resumed"})
}

// Remove 移除任务
func (h *Handler) Remove(c *gin.Context) {
	gid := c.Param("gid")
	force := c.Query("force") == "true"
	if err := h.service.Remove(gid, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task removed"})
}

// RemoveResult 移除下载结果
func (h *Handler) RemoveResult(c *gin.Context) {
	gid := c.Param("gid")
	if err := h.service.RemoveResult(gid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "result removed"})
}

// PauseAll 暂停所有
func (h *Handler) PauseAll(c *gin.Context) {
	if err := h.service.PauseAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "all tasks paused"})
}

// ResumeAll 恢复所有
func (h *Handler) ResumeAll(c *gin.Context) {
	if err := h.service.ResumeAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "all tasks resumed"})
}

// PurgeResults 清除所有结果
func (h *Handler) PurgeResults(c *gin.Context) {
	if err := h.service.PurgeResults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "results purged"})
}

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// UpdateOptions 更新选项
func (h *Handler) UpdateOptions(c *gin.Context) {
	var req UpdateOptionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateOptions(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "options updated"})
}

// Start 启动 aria2
func (h *Handler) Start(c *gin.Context) {
	if err := h.service.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "aria2 started"})
}

// Stop 停止 aria2
func (h *Handler) Stop(c *gin.Context) {
	h.service.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "aria2 stopped"})
}

// WebSocket 处理 WebSocket 连接
func (h *Handler) WebSocket(c *gin.Context) {
	hub := h.service.GetHub()
	if hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "websocket not available"})
		return
	}
	hub.ServeWs(c.Writer, c.Request)
}

// GetHistory 获取下载历史
func (h *Handler) GetHistory(c *gin.Context) {
	limit := 50
	offset := 0
	status := c.Query("status")

	if l := c.Query("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := c.Query("offset"); o != "" {
		if n, err := parseInt(o); err == nil && n >= 0 {
			offset = n
		}
	}

	history, total, err := h.service.GetHistory(limit, offset, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": history,
		"total": total,
	})
}

// SearchHistory 搜索历史记录
func (h *Handler) SearchHistory(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing search keyword"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 {
			limit = n
		}
	}

	history, err := h.service.SearchHistory(keyword, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": history})
}

// DeleteHistoryItem 删除单条历史记录
func (h *Handler) DeleteHistoryItem(c *gin.Context) {
	idStr := c.Param("id")
	id, err := parseInt(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.DeleteHistoryItem(int64(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "history item deleted"})
}

// ClearHistory 清空历史记录
func (h *Handler) ClearHistory(c *gin.Context) {
	if err := h.service.ClearHistory(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "history cleared"})
}

// GetSettings 获取用户设置
func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.service.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings 更新用户设置
func (h *Handler) UpdateSettings(c *gin.Context) {
	var settings UserSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateSettings(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}

// GetStatistics 获取下载统计
func (h *Handler) GetStatistics(c *gin.Context) {
	stats, err := h.service.GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// parseInt 解析整数
func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
