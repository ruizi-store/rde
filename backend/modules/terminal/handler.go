// Package terminal 终端模块 - HTTP/WebSocket 处理器
package terminal

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/ruizi-store/rde/backend/core/auth"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	service  *Service
	upgrader websocket.Upgrader
	logger   *zap.Logger
	dataPath string // 数据目录，用于读取配置
}

// NewHandler 创建处理器
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service:  service,
		logger:   logger,
		dataPath: "/var/lib/rde/data", // 默认路径
		upgrader: websocket.Upgrader{
			ReadBufferSize:  WebSocketBufferSize,
			WriteBufferSize: WebSocketBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				// 在生产环境中应该验证 Origin
				return true
			},
		},
	}
}

// SetDataPath 设置数据目录
func (h *Handler) SetDataPath(path string) {
	h.dataPath = path
}

// isTerminalEnabled 检查终端是否启用
func (h *Handler) isTerminalEnabled() bool {
	configFile := filepath.Join(h.dataPath, "remote_access.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false // 默认关闭
	}

	var settings struct {
		TerminalEnabled bool `json:"terminal_enabled"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	return settings.TerminalEnabled
}

// checkTerminalEnabled 检查终端是否启用的中间件
func (h *Handler) checkTerminalEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.isTerminalEnabled() {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "终端功能未启用，请在系统设置中启用",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup, tokenManager *auth.TokenManager) {
	// 所有终端路由都需要管理员权限和终端启用检查
	terminal := router.Group("/terminal")
	terminal.Use(auth.Middleware(tokenManager), auth.RequireAdmin(), h.checkTerminalEnabled())
	{
		terminal.POST("", h.CreateSession)
		terminal.GET("", h.ListSessions)
		terminal.DELETE("/:id", h.CloseSession)
		terminal.POST("/:id/resize", h.Resize)
	}

	// WebSocket 端点需要单独处理认证（因为 WebSocket 握手时 header 处理不同）
	// 同时也需要检查终端是否启用
	router.GET("/terminal/:id/ws", h.handleWebSocketAuth(tokenManager), h.checkTerminalEnabled(), h.WebSocket)
}

// handleWebSocketAuth WebSocket 认证中间件
func (h *Handler) handleWebSocketAuth(tokenManager *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先从 query 参数获取 token
		token := c.Query("token")
		if token == "" {
			// 尝试从 header 获取
			token = auth.ExtractToken(c)
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required",
			})
			c.Abort()
			return
		}

		// 验证 token
		claims, err := tokenManager.ParseAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid token",
			})
			c.Abort()
			return
		}

		// 检查是否为管理员
		if claims.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Admin access required",
			})
			c.Abort()
			return
		}

		// 设置上下文
		c.Set(auth.ContextKeyUserID, claims.UserID)
		c.Set(auth.ContextKeyUsername, claims.Username)
		c.Set(auth.ContextKeyRole, claims.Role)
		c.Set(auth.ContextKeyClaims, claims)

		c.Next()
	}
}

// CreateSession 创建终端会话
// @Summary 创建终端会话
// @Tags Terminal
// @Accept json
// @Produce json
// @Param request body CreateSessionRequest true "创建请求"
// @Success 200 {object} map[string]interface{}
// @Router /terminal [post]
func (h *Handler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有 body，使用默认值
		req = CreateSessionRequest{}
	}

	userID := c.GetUint(auth.ContextKeyUserID)
	username := c.GetString(auth.ContextKeyUsername)

	session, err := h.service.CreateSession(userID, username, &req)
	if err != nil {
		h.logger.Error("Failed to create terminal session",
			zap.Error(err),
			zap.Uint("userId", userID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    session.ToInfo(),
	})
}

// ListSessions 获取会话列表
// @Summary 获取终端会话列表
// @Tags Terminal
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /terminal [get]
func (h *Handler) ListSessions(c *gin.Context) {
	// 管理员可以看到所有会话
	sessions := h.service.GetAllSessions()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sessions,
	})
}

// CloseSession 关闭会话
// @Summary 关闭终端会话
// @Tags Terminal
// @Param id path string true "会话 ID"
// @Success 200 {object} map[string]interface{}
// @Router /terminal/{id} [delete]
func (h *Handler) CloseSession(c *gin.Context) {
	sessionID := c.Param("id")

	if err := h.service.CloseSession(sessionID); err != nil {
		h.logger.Warn("Failed to close terminal session",
			zap.Error(err),
			zap.String("sessionId", sessionID),
		)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session closed",
	})
}

// Resize 调整终端大小
// @Summary 调整终端大小
// @Tags Terminal
// @Accept json
// @Produce json
// @Param id path string true "会话 ID"
// @Param request body ResizeRequest true "调整请求"
// @Success 200 {object} map[string]interface{}
// @Router /terminal/{id}/resize [post]
func (h *Handler) Resize(c *gin.Context) {
	sessionID := c.Param("id")

	var req ResizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	if err := h.service.Resize(sessionID, req.Cols, req.Rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Resized",
	})
}

// WebSocket 处理 WebSocket 连接
// @Summary WebSocket 终端连接
// @Tags Terminal
// @Param id path string true "会话 ID"
// @Param token query string false "JWT Token"
// @Router /terminal/{id}/ws [get]
func (h *Handler) WebSocket(c *gin.Context) {
	sessionID := c.Param("id")

	// 检查会话是否存在
	session, ok := h.service.GetSession(sessionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Session not found",
		})
		return
	}

	// 验证会话所有权（管理员可以访问所有会话）
	_ = session // 管理员已通过中间件验证，可访问所有会话

	// 升级到 WebSocket
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed",
			zap.Error(err),
			zap.String("sessionId", sessionID),
		)
		return
	}
	defer ws.Close()

	h.logger.Info("WebSocket connected",
		zap.String("sessionId", sessionID),
		zap.String("remoteAddr", c.ClientIP()),
	)

	// 附加到会话
	if err := h.service.AttachWebSocket(sessionID, ws); err != nil {
		h.logger.Error("Failed to attach WebSocket",
			zap.Error(err),
			zap.String("sessionId", sessionID),
		)
		return
	}

	h.logger.Info("WebSocket disconnected",
		zap.String("sessionId", sessionID),
	)
}
