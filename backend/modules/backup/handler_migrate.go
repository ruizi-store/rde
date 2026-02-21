// Package backup 提供备份还原功能
package backup

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MigrateHandler 迁移 HTTP 处理器
type MigrateHandler struct {
	p2p *P2PMigrateService
}

// NewMigrateHandler 创建迁移处理器
func NewMigrateHandler(p2p *P2PMigrateService) *MigrateHandler {
	return &MigrateHandler{p2p: p2p}
}

// RegisterMigrateRoutes 注册迁移路由
func (h *MigrateHandler) RegisterMigrateRoutes(r *gin.RouterGroup) {
	migrate := r.Group("/backup/migrate")
	{
		// 生成配对码（目标端调用）
		migrate.POST("/pair", h.GeneratePairCode)
		
		// 验证配对码
		migrate.POST("/validate", h.ValidatePairCode)
		
		// 连接到源（源端调用）
		migrate.POST("/connect", h.ConnectToSource)
		
		// 获取会话状态
		migrate.GET("/session/:id", h.GetSession)
		
		// 获取迁移进度
		migrate.GET("/session/:id/progress", h.GetProgress)
		
		// 开始传输（源端调用）
		migrate.POST("/session/:id/start", h.StartTransfer)
		
		// 取消迁移
		migrate.POST("/session/:id/cancel", h.CancelSession)
		
		// WebSocket 传输通道
		migrate.GET("/ws/:pairCode", h.WebSocketHandler)
	}
}

// GeneratePairCodeRequest 生成配对码请求
type GeneratePairCodeRequest struct{}

// GeneratePairCodeResponse 生成配对码响应
type GeneratePairCodeResponse struct {
	SessionID string `json:"session_id"`
	PairCode  string `json:"pair_code"`
	ExpiresAt string `json:"expires_at"`
}

// GeneratePairCode 生成配对码
// @Summary 生成配对码
// @Description 目标端调用，生成用于配对的临时码
// @Tags migrate
// @Accept json
// @Produce json
// @Success 200 {object} GeneratePairCodeResponse
// @Router /backup/migrate/pair [post]
func (h *MigrateHandler) GeneratePairCode(c *gin.Context) {
	session, err := h.p2p.GeneratePairCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": GeneratePairCodeResponse{
			SessionID: session.ID,
			PairCode:  session.PairCode,
			ExpiresAt: session.ExpiresAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// ValidatePairCodeRequest 验证配对码请求
type ValidatePairCodeRequest struct {
	PairCode string `json:"pair_code" binding:"required"`
}

// ValidatePairCode 验证配对码
// @Summary 验证配对码
// @Description 检查配对码是否有效
// @Tags migrate
// @Accept json
// @Produce json
// @Param request body ValidatePairCodeRequest true "配对码"
// @Success 200 {object} map[string]interface{}
// @Router /backup/migrate/validate [post]
func (h *MigrateHandler) ValidatePairCode(c *gin.Context) {
	var req ValidatePairCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	session, err := h.p2p.ValidatePairCode(req.PairCode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"valid":      true,
			"session_id": session.ID,
			"expires_at": session.ExpiresAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// ConnectToSourceRequest 连接到源请求
type ConnectToSourceRequest struct {
	PairCode  string `json:"pair_code" binding:"required"`
	TargetURL string `json:"target_url" binding:"required"`
}

// ConnectToSource 连接到源
// @Summary 连接到数据源
// @Description 源端调用，使用配对码连接到目标端
// @Tags migrate
// @Accept json
// @Produce json
// @Param request body ConnectToSourceRequest true "连接请求"
// @Success 200 {object} map[string]interface{}
// @Router /backup/migrate/connect [post]
func (h *MigrateHandler) ConnectToSource(c *gin.Context) {
	var req ConnectToSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	session, err := h.p2p.ConnectWithPairCode(req.PairCode, req.TargetURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"session_id": session.ID,
			"status":     session.Status,
		},
	})
}

// GetSession 获取会话状态
// @Summary 获取会话状态
// @Description 获取迁移会话的当前状态
// @Tags migrate
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {object} map[string]interface{}
// @Router /backup/migrate/session/{id} [get]
func (h *MigrateHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")

	session, err := h.p2p.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":          session.ID,
			"pair_code":   session.PairCode,
			"role":        session.Role,
			"status":      session.Status,
			"remote_addr": session.RemoteAddr,
			"remote_host": session.RemoteHost,
			"created_at":  session.CreatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// GetProgress 获取迁移进度
// @Summary 获取迁移进度
// @Description 获取当前迁移任务的详细进度
// @Tags migrate
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {object} MigrateProgress
// @Router /backup/migrate/session/{id}/progress [get]
func (h *MigrateHandler) GetProgress(c *gin.Context) {
	sessionID := c.Param("id")

	progress, err := h.p2p.GetProgress(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    progress,
	})
}

// StartTransferRequest 开始传输请求
type StartTransferRequest struct {
	SystemConfig bool     `json:"system_config"`
	Users        bool     `json:"users"`
	Docker       bool     `json:"docker"`
	Network      bool     `json:"network"`
	Samba        bool     `json:"samba"`
	Files        []string `json:"files"`
	Apps         []string `json:"apps"`
}

// StartTransfer 开始传输
// @Summary 开始数据传输
// @Description 源端调用，选择要迁移的内容并开始传输
// @Tags migrate
// @Accept json
// @Produce json
// @Param id path string true "会话ID"
// @Param request body StartTransferRequest true "要迁移的内容"
// @Success 200 {object} map[string]interface{}
// @Router /backup/migrate/session/{id}/start [post]
func (h *MigrateHandler) StartTransfer(c *gin.Context) {
	sessionID := c.Param("id")

	var req StartTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	content := &MigrateContentSelection{
		SystemConfig: req.SystemConfig,
		Users:        req.Users,
		Docker:       req.Docker,
		Network:      req.Network,
		Samba:        req.Samba,
		Files:        req.Files,
		Apps:         req.Apps,
	}

	if err := h.p2p.StartTransfer(sessionID, content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "传输已开始",
	})
}

// CancelSession 取消会话
// @Summary 取消迁移会话
// @Description 取消正在进行的迁移
// @Tags migrate
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {object} map[string]interface{}
// @Router /backup/migrate/session/{id}/cancel [post]
func (h *MigrateHandler) CancelSession(c *gin.Context) {
	sessionID := c.Param("id")

	if err := h.p2p.CancelSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "迁移已取消",
	})
}

// WebSocketHandler WebSocket 处理
// @Summary WebSocket 传输通道
// @Description 建立 P2P 数据传输的 WebSocket 连接
// @Tags migrate
// @Param pairCode path string true "配对码"
// @Router /backup/migrate/ws/{pairCode} [get]
func (h *MigrateHandler) WebSocketHandler(c *gin.Context) {
	pairCode := c.Param("pairCode")

	if err := h.p2p.HandleWebSocket(c.Writer, c.Request, pairCode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
}
