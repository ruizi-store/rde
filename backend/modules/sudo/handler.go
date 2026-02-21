package sudo

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	executor *Executor
	logger   *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(executor *Executor, logger *zap.Logger) *Handler {
	return &Handler{
		executor: executor,
		logger:   logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	sudo := group.Group("/sudo")
	{
		sudo.GET("/actions", h.ListActions)
		sudo.POST("/preview", h.Preview)
		sudo.POST("/execute", h.Execute)
		sudo.GET("/logs", h.GetLogs)
	}
}

// ListActions 列出所有可用操作
// @Summary 列出所有可用的 sudo 操作
// @Tags sudo
// @Produce json
// @Success 200 {array} ActionInfo
// @Router /api/v1/sudo/actions [get]
func (h *Handler) ListActions(c *gin.Context) {
	actions := h.executor.GetAllActions()
	infos := make([]ActionInfo, len(actions))
	for i, action := range actions {
		infos[i] = ActionInfo{
			ID:          action.ID,
			Name:        action.Name,
			Description: action.Description,
			ArgCount:    action.ArgCount,
			Dangerous:   action.Dangerous,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    infos,
	})
}

// Preview 预览将要执行的命令
// @Summary 预览 sudo 命令
// @Tags sudo
// @Accept json
// @Produce json
// @Param request body PreviewRequest true "预览请求"
// @Success 200 {object} PreviewResponse
// @Router /api/v1/sudo/preview [post]
func (h *Handler) Preview(c *gin.Context) {
	var req PreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	action := h.executor.GetAction(req.ActionID)
	if action == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "unknown action: " + req.ActionID,
		})
		return
	}

	cmdStr, err := h.executor.BuildCommand(action, req.Args)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": PreviewResponse{
			Action: ActionInfo{
				ID:          action.ID,
				Name:        action.Name,
				Description: action.Description,
				ArgCount:    action.ArgCount,
				Dangerous:   action.Dangerous,
			},
			Command: "sudo " + cmdStr,
		},
	})
}

// Execute 执行 sudo 命令
// @Summary 执行 sudo 命令
// @Tags sudo
// @Accept json
// @Produce json
// @Param request body ExecuteRequest true "执行请求"
// @Success 200 {object} ExecuteResponse
// @Router /api/v1/sudo/execute [post]
func (h *Handler) Execute(c *gin.Context) {
	var req ExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	// 获取用户信息
	userID := fmt.Sprintf("%d", auth.GetUserID(c))
	username := auth.GetUsername(c)
	clientIP := c.ClientIP()

	resp, err := h.executor.Execute(c.Request.Context(), &req, userID, username, clientIP)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"data":    resp,
	})
}

// GetLogs 获取审计日志
// @Summary 获取 sudo 审计日志
// @Tags sudo
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {array} AuditLog
// @Router /api/v1/sudo/logs [get]
func (h *Handler) GetLogs(c *gin.Context) {
	// 仅管理员可查看
	if !auth.IsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "admin required",
		})
		return
	}

	page := parseInt(c.DefaultQuery("page", "1"))
	size := parseInt(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	var logs []AuditLog
	var total int64

	h.executor.db.Model(&AuditLog{}).Count(&total)
	h.executor.db.Order("timestamp DESC").Offset((page - 1) * size).Limit(size).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": logs,
			"total": total,
			"page":  page,
			"size":  size,
		},
	})
}

func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
