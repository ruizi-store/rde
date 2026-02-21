// Package notification HTTP 处理器
package notification

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
)

// Handler HTTP 处理器
type Handler struct {
	service      *Service
	tokenManager *auth.TokenManager
}

// NewHandler 创建处理器
func NewHandler(service *Service, tokenManager *auth.TokenManager) *Handler {
	return &Handler{service: service, tokenManager: tokenManager}
}

// response 标准响应
type response struct {
	Success int         `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listResponse 列表响应
type listResponse struct {
	Success int         `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Total   int64       `json:"total,omitempty"`
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, response{
		Success: 200,
		Message: "success",
		Data:    data,
	})
}

func okList(c *gin.Context, data interface{}, total int64) {
	c.JSON(http.StatusOK, listResponse{
		Success: 200,
		Message: "success",
		Data:    data,
		Total:   total,
	})
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, response{
		Success: code,
		Message: msg,
	})
}

// ==================== 站内通知 ====================

// ListNotifications 获取通知列表
// @Summary 获取通知列表
// @Tags notifications
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param category query string false "类别（逗号分隔）"
// @Param severity query string false "级别（逗号分隔）"
// @Param is_read query bool false "是否已读"
// @Success 200 {object} NotificationListResponse
// @Router /api/v1/notifications [get]
func (h *Handler) ListNotifications(c *gin.Context) {
	userID := c.GetString("user_id")

	var req ListNotificationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	result, err := h.service.ListNotifications(c.Request.Context(), userID, &req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}

	ok(c, result)
}

// GetUnreadCount 获取未读数量
// @Summary 获取未读数量
// @Tags notifications
// @Produce json
// @Success 200 {object} UnreadCountResponse
// @Router /api/v1/notifications/unread-count [get]
func (h *Handler) GetUnreadCount(c *gin.Context) {
	userID := c.GetString("user_id")
	count, err := h.service.GetUnreadCount(c.Request.Context(), userID)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, UnreadCountResponse{Count: count})
}

// MarkAsRead 标记为已读
// @Summary 标记单条通知为已读
// @Tags notifications
// @Produce json
// @Param id path string true "通知ID"
// @Success 200 {object} response
// @Router /api/v1/notifications/{id}/read [put]
func (h *Handler) MarkAsRead(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	if err := h.service.MarkAsRead(c.Request.Context(), userID, id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// MarkAllAsRead 全部标记已读
// @Summary 标记所有通知为已读
// @Tags notifications
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/notifications/read-all [put]
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	userID := c.GetString("user_id")

	if err := h.service.MarkAllAsRead(c.Request.Context(), userID); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// DeleteNotification 删除单条通知
// @Summary 删除单条通知
// @Tags notifications
// @Produce json
// @Param id path string true "通知ID"
// @Success 200 {object} response
// @Router /api/v1/notifications/{id} [delete]
func (h *Handler) DeleteNotification(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	if err := h.service.DeleteNotification(c.Request.Context(), userID, id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// DeleteReadNotifications 删除已读通知
// @Summary 删除已读通知
// @Tags notifications
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/notifications/read [delete]
func (h *Handler) DeleteReadNotifications(c *gin.Context) {
	userID := c.GetString("user_id")

	if err := h.service.DeleteReadNotifications(c.Request.Context(), userID); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// DeleteAllNotifications 删除所有通知
// @Summary 删除所有通知
// @Tags notifications
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/notifications [delete]
func (h *Handler) DeleteAllNotifications(c *gin.Context) {
	userID := c.GetString("user_id")

	if err := h.service.DeleteAllNotifications(c.Request.Context(), userID); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// ==================== 通知设置 ====================

// GetSettings 获取通知设置
// @Summary 获取通知设置
// @Tags notifications
// @Produce json
// @Success 200 {object} SettingsResponse
// @Router /api/v1/notifications/settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	userID := c.GetString("user_id")

	settings, err := h.service.GetSettings(c.Request.Context(), userID)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, settings)
}

// UpdateSettings 更新通知设置
// @Summary 更新通知设置
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body UpdateSettingsRequest true "设置"
// @Success 200 {object} SettingsResponse
// @Router /api/v1/notifications/settings [put]
func (h *Handler) UpdateSettings(c *gin.Context) {
	userID := c.GetString("user_id")

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	settings, err := h.service.UpdateSettings(c.Request.Context(), userID, &req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, settings)
}

// ==================== 推送渠道 ====================

// CreateChannel 创建渠道
// @Summary 创建通知渠道
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body CreateChannelRequest true "渠道信息"
// @Success 200 {object} NotificationChannel
// @Router /api/v1/notifications/channels [post]
func (h *Handler) CreateChannel(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	channel, err := h.service.CreateChannel(c.Request.Context(), userID, &req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, channel)
}

// GetChannel 获取渠道
// @Summary 获取通知渠道
// @Tags notifications
// @Produce json
// @Param id path string true "渠道ID"
// @Success 200 {object} NotificationChannel
// @Router /api/v1/notifications/channels/{id} [get]
func (h *Handler) GetChannel(c *gin.Context) {
	id := c.Param("id")
	channel, err := h.service.GetChannel(c.Request.Context(), id)
	if err != nil {
		fail(c, 404, "渠道不存在")
		return
	}
	ok(c, channel)
}

// ListChannels 列出渠道
// @Summary 列出所有通知渠道
// @Tags notifications
// @Produce json
// @Success 200 {array} NotificationChannel
// @Router /api/v1/notifications/channels [get]
func (h *Handler) ListChannels(c *gin.Context) {
	userID := c.GetString("user_id")
	channels, err := h.service.ListChannels(c.Request.Context(), userID)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, channels)
}

// UpdateChannel 更新渠道
// @Summary 更新通知渠道
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path string true "渠道ID"
// @Param request body UpdateChannelRequest true "更新信息"
// @Success 200 {object} NotificationChannel
// @Router /api/v1/notifications/channels/{id} [put]
func (h *Handler) UpdateChannel(c *gin.Context) {
	id := c.Param("id")
	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	channel, err := h.service.UpdateChannel(c.Request.Context(), id, &req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, channel)
}

// DeleteChannel 删除渠道
// @Summary 删除通知渠道
// @Tags notifications
// @Produce json
// @Param id path string true "渠道ID"
// @Success 200 {object} response
// @Router /api/v1/notifications/channels/{id} [delete]
func (h *Handler) DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteChannel(c.Request.Context(), id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// TestChannel 测试渠道
// @Summary 测试通知渠道
// @Tags notifications
// @Produce json
// @Param id path string true "渠道ID"
// @Success 200 {object} response
// @Router /api/v1/notifications/channels/{id}/test [post]
func (h *Handler) TestChannel(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.TestChannel(c.Request.Context(), id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// ==================== 推送规则 ====================

// CreateRule 创建规则
// @Summary 创建通知规则
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body CreateRuleRequest true "规则信息"
// @Success 200 {object} NotificationRule
// @Router /api/v1/notifications/rules [post]
func (h *Handler) CreateRule(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	rule, err := h.service.CreateRule(c.Request.Context(), userID, &req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, rule)
}

// GetRule 获取规则
// @Summary 获取通知规则
// @Tags notifications
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} NotificationRule
// @Router /api/v1/notifications/rules/{id} [get]
func (h *Handler) GetRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.service.GetRule(c.Request.Context(), id)
	if err != nil {
		fail(c, 404, "规则不存在")
		return
	}
	ok(c, rule)
}

// ListRules 列出规则
// @Summary 列出所有通知规则
// @Tags notifications
// @Produce json
// @Success 200 {array} NotificationRule
// @Router /api/v1/notifications/rules [get]
func (h *Handler) ListRules(c *gin.Context) {
	userID := c.GetString("user_id")
	rules, err := h.service.ListRules(c.Request.Context(), userID)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, rules)
}

// UpdateRule 更新规则
// @Summary 更新通知规则
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path string true "规则ID"
// @Param request body UpdateRuleRequest true "更新信息"
// @Success 200 {object} NotificationRule
// @Router /api/v1/notifications/rules/{id} [put]
func (h *Handler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	rule, err := h.service.UpdateRule(c.Request.Context(), id, &req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, rule)
}

// DeleteRule 删除规则
// @Summary 删除通知规则
// @Tags notifications
// @Produce json
// @Param id path string true "规则ID"
// @Success 200 {object} response
// @Router /api/v1/notifications/rules/{id} [delete]
func (h *Handler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteRule(c.Request.Context(), id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// ==================== 推送历史 ====================

// GetHistory 获取推送历史
// @Summary 获取推送历史
// @Tags notifications
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {array} NotificationHistory
// @Router /api/v1/notifications/history [get]
func (h *Handler) GetHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	history, total, err := h.service.GetHistory(c.Request.Context(), page, pageSize)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	okList(c, history, total)
}

// ==================== WebSocket ====================

// WebSocketHandler WebSocket 连接处理
func (h *Handler) WebSocketHandler(c *gin.Context) {
	// WebSocket 连接无法使用 Authorization header，需要从查询参数获取 token
	token := c.Query("token")
	if token == "" {
		token = auth.ExtractToken(c)
	}

	var userID string
	if token != "" && h.tokenManager != nil {
		claims, err := h.tokenManager.ParseAccessToken(token)
		if err == nil {
			userID = claims.UserID
		}
	}

	// userID 可以为空，表示未认证的广播连接
	h.service.GetHub().HandleWebSocket(c.Writer, c.Request, userID)
}

// ==================== 路由注册 ====================

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	// 站内通知 - 使用 /notifications 路径
	notifications := group.Group("/notifications")
	{
		notifications.GET("", h.ListNotifications)
		notifications.GET("/unread-count", h.GetUnreadCount)
		notifications.PUT("/:id/read", h.MarkAsRead)
		notifications.PUT("/read-all", h.MarkAllAsRead)
		notifications.DELETE("/:id", h.DeleteNotification)
		notifications.DELETE("/read", h.DeleteReadNotifications)
		notifications.DELETE("", h.DeleteAllNotifications)

		// 设置
		notifications.GET("/settings", h.GetSettings)
		notifications.PUT("/settings", h.UpdateSettings)

		// 渠道
		channels := notifications.Group("/channels")
		{
			channels.POST("", h.CreateChannel)
			channels.GET("", h.ListChannels)
			channels.GET("/:id", h.GetChannel)
			channels.PUT("/:id", h.UpdateChannel)
			channels.DELETE("/:id", h.DeleteChannel)
			channels.POST("/:id/test", h.TestChannel)
		}

		// 规则
		rules := notifications.Group("/rules")
		{
			rules.POST("", h.CreateRule)
			rules.GET("", h.ListRules)
			rules.GET("/:id", h.GetRule)
			rules.PUT("/:id", h.UpdateRule)
			rules.DELETE("/:id", h.DeleteRule)
		}

		// 历史
		notifications.GET("/history", h.GetHistory)

		// WebSocket
		notifications.GET("/ws", h.WebSocketHandler)
	}
}
