// Package samba 提供 Samba HTTP 处理器
package samba

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler Samba API 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r
	{
		// 服务状态
		g.GET("/status", h.GetStatus)
		g.POST("/start", h.StartService)
		g.POST("/stop", h.StopService)
		g.POST("/restart", h.RestartService)
		g.POST("/reload", h.ReloadService)
		g.GET("/test", h.TestConfig)

		// 全局配置
		g.GET("/config", h.GetGlobalConfig)
		g.PUT("/config", h.UpdateGlobalConfig)

		// 共享管理
		g.GET("/shares", h.ListShares)
		g.GET("/shares/:name", h.GetShare)
		g.POST("/shares", h.CreateShare)
		g.PUT("/shares/:name", h.UpdateShare)
		g.DELETE("/shares/:name", h.DeleteShare)

		// 用户管理
		g.GET("/users", h.ListUsers)
		g.POST("/users", h.AddUser)
		g.DELETE("/users/:username", h.DeleteUser)
		g.PUT("/users/:username/password", h.SetUserPassword)
		g.GET("/system-users", h.GetSystemUsers)

		// 会话管理
		g.GET("/sessions", h.GetSessions)
		g.DELETE("/sessions/:pid", h.KillSession)
		g.DELETE("/sessions/user/:username", h.KillUserSessions)
	}
}

// ==================== 服务状态 ====================

// GetStatus 获取服务状态
func (h *Handler) GetStatus(c *gin.Context) {
	status, err := h.service.GetServiceStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// StartService 启动服务
func (h *Handler) StartService(c *gin.Context) {
	if err := h.service.StartService(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "service started"})
}

// StopService 停止服务
func (h *Handler) StopService(c *gin.Context) {
	if err := h.service.StopService(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "service stopped"})
}

// RestartService 重启服务
func (h *Handler) RestartService(c *gin.Context) {
	if err := h.service.RestartService(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "service restarted"})
}

// ReloadService 重载配置
func (h *Handler) ReloadService(c *gin.Context) {
	if err := h.service.ReloadService(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config reloaded"})
}

// TestConfig 测试配置
func (h *Handler) TestConfig(c *gin.Context) {
	output, err := h.service.TestConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "output": output})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "output": output})
}

// ==================== 全局配置 ====================

// GetGlobalConfig 获取全局配置
func (h *Handler) GetGlobalConfig(c *gin.Context) {
	config, err := h.service.GetGlobalConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

// UpdateGlobalConfig 更新全局配置
func (h *Handler) UpdateGlobalConfig(c *gin.Context) {
	var req UpdateGlobalConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateGlobalConfig(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// ==================== 共享管理 ====================

// ListShares 列出共享
func (h *Handler) ListShares(c *gin.Context) {
	shares, err := h.service.ListShares(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, shares)
}

// GetShare 获取共享详情
func (h *Handler) GetShare(c *gin.Context) {
	name := c.Param("name")
	share, err := h.service.GetShare(c.Request.Context(), name)
	if err != nil {
		if err.Error() == "share '"+name+"' not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, share)
}

// CreateShare 创建共享
func (h *Handler) CreateShare(c *gin.Context) {
	var req CreateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	share, err := h.service.CreateShare(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, share)
}

// UpdateShare 更新共享
func (h *Handler) UpdateShare(c *gin.Context) {
	name := c.Param("name")
	var req UpdateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	share, err := h.service.UpdateShare(c.Request.Context(), name, req)
	if err != nil {
		if err.Error() == "share '"+name+"' not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, share)
}

// DeleteShare 删除共享
func (h *Handler) DeleteShare(c *gin.Context) {
	name := c.Param("name")
	if err := h.service.DeleteShare(c.Request.Context(), name); err != nil {
		if err.Error() == "share '"+name+"' not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "share deleted"})
}

// ==================== 用户管理 ====================

// ListUsers 列出用户
func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.service.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

// AddUser 添加用户
func (h *Handler) AddUser(c *gin.Context) {
	var req AddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	if err := h.service.AddUser(c.Request.Context(), req.Username, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "user added"})
}

// DeleteUser 删除用户
func (h *Handler) DeleteUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.service.DeleteUser(c.Request.Context(), username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

// SetUserPassword 设置用户密码
func (h *Handler) SetUserPassword(c *gin.Context) {
	username := c.Param("username")
	var req SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password required"})
		return
	}

	if err := h.service.SetUserPassword(c.Request.Context(), username, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

// GetSystemUsers 获取系统用户
func (h *Handler) GetSystemUsers(c *gin.Context) {
	users, err := h.service.GetSystemUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

// ==================== 会话管理 ====================

// GetSessions 获取会话
func (h *Handler) GetSessions(c *gin.Context) {
	sessions, err := h.service.GetSessions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sessions)
}

// KillSession 终止会话
func (h *Handler) KillSession(c *gin.Context) {
	pidStr := c.Param("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pid"})
		return
	}

	if err := h.service.KillSession(c.Request.Context(), pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "session killed"})
}

// KillUserSessions 终止用户会话
func (h *Handler) KillUserSessions(c *gin.Context) {
	username := c.Param("username")
	if err := h.service.KillUserSessions(c.Request.Context(), username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user sessions killed"})
}
