// Package docker HTTP 处理器
package docker

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// Docker 信息
	r.GET("/info", h.GetInfo)
	r.GET("/status", h.GetStatus)

	// 镜像管理
	r.GET("/images", h.ListImages)
	r.POST("/images/pull", h.PullImage)
	r.DELETE("/images/:id", h.RemoveImage)

	// 容器管理
	r.GET("/containers", h.ListContainers)
	r.POST("/containers", h.CreateContainer)
	r.GET("/containers/:id", h.GetContainerStatus)
	r.DELETE("/containers/:id", h.RemoveContainer)
	r.POST("/containers/:id/start", h.StartContainer)
	r.POST("/containers/:id/stop", h.StopContainer)
	r.POST("/containers/:id/restart", h.RestartContainer)
	r.GET("/containers/:id/stats", h.GetContainerStats)
	r.GET("/containers/:id/logs", h.GetContainerLogs)
	r.POST("/containers/:id/exec", h.ExecContainer)

	// 网络管理
	r.GET("/networks", h.ListNetworks)
	r.POST("/networks", h.CreateNetwork)
	r.DELETE("/networks/:id", h.RemoveNetwork)
}

// GetStatus 获取 Docker 运行状态
func (h *Handler) GetStatus(c *gin.Context) {
	running := false
	if h.service != nil {
		running = h.service.IsRunning(c.Request.Context())
	}
	c.JSON(http.StatusOK, gin.H{
		"running": running,
	})
}

// GetInfo 获取 Docker 信息
func (h *Handler) GetInfo(c *gin.Context) {
	if h.service == nil || !h.service.IsRunning(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Docker is not running"})
		return
	}

	info, err := h.service.GetInfo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// ListImages 列出镜像
func (h *Handler) ListImages(c *gin.Context) {
	images, err := h.service.ListImages(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": images})
}

// PullImage 拉取镜像
func (h *Handler) PullImage(c *gin.Context) {
	var req PullImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.PullImage(c.Request.Context(), req.Image, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Image pulled successfully"})
}

// RemoveImage 删除镜像
func (h *Handler) RemoveImage(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.RemoveImage(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Image removed"})
}

// ListContainers 列出容器
func (h *Handler) ListContainers(c *gin.Context) {
	all := c.Query("all") == "true"
	containers, err := h.service.ListContainers(c.Request.Context(), all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": containers})
}

// CreateContainer 创建容器
func (h *Handler) CreateContainer(c *gin.Context) {
	var req CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &ContainerConfig{
		Name:        req.Name,
		Image:       req.Image,
		Ports:       req.Ports,
		Volumes:     req.Volumes,
		Environment: req.Environment,
		Networks:    req.Networks,
		Labels:      req.Labels,
		Restart:     req.Restart,
		Privileged:  req.Privileged,
		CapAdd:      req.CapAdd,
		Devices:     req.Devices,
		Command:     req.Command,
	}

	id, err := h.service.CreateContainer(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id}})
}

// GetContainerStatus 获取容器状态
func (h *Handler) GetContainerStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := h.service.GetContainerStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// RemoveContainer 删除容器
func (h *Handler) RemoveContainer(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"
	if err := h.service.RemoveContainer(c.Request.Context(), id, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Container removed"})
}

// StartContainer 启动容器
func (h *Handler) StartContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.StartContainer(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to start container", zap.Error(err), zap.String("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Container started"})
}

// StopContainer 停止容器
func (h *Handler) StopContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.StopContainer(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to stop container", zap.Error(err), zap.String("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Container stopped"})
}

// RestartContainer 重启容器
func (h *Handler) RestartContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.RestartContainer(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to restart container", zap.Error(err), zap.String("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Container restarted"})
}

// GetContainerStats 获取容器统计
func (h *Handler) GetContainerStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.service.GetContainerStats(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetContainerLogs 获取容器日志
func (h *Handler) GetContainerLogs(c *gin.Context) {
	id := c.Param("id")
	tail := 100
	if t := c.Query("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil {
			tail = n
		}
	}

	logs, err := h.service.GetContainerLogs(c.Request.Context(), id, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// ExecContainer 在容器内执行命令
func (h *Handler) ExecContainer(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}

	output, err := h.service.ExecInContainer(c.Request.Context(), id, req.Command)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"output": output}})
}

// ListNetworks 列出网络
func (h *Handler) ListNetworks(c *gin.Context) {
	networks, err := h.service.ListNetworks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": networks})
}

// CreateNetwork 创建网络
func (h *Handler) CreateNetwork(c *gin.Context) {
	var req CreateNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := h.service.CreateNetwork(c.Request.Context(), req.Name, req.Driver)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id}})
}

// RemoveNetwork 删除网络
func (h *Handler) RemoveNetwork(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.RemoveNetwork(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Network removed"})
}
