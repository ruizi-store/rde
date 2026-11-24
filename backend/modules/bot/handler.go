// Package bot HTTP 处理器
package bot

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
	proxy   *StreamProxy
	skills  *SkillService
}

// NewHandler 创建处理器
func NewHandler(service *Service, proxy *StreamProxy, skills *SkillService) *Handler {
	return &Handler{service: service, proxy: proxy, skills: skills}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/bots")
	{
		g.GET("", h.List)
		g.POST("", h.Create)
		g.GET("/:id", h.Get)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)

		g.POST("/:id/start", h.Start)
		g.POST("/:id/stop", h.Stop)
		g.POST("/:id/restart", h.Restart)
		g.POST("/:id/exec", h.Exec)
		g.GET("/:id/logs", h.Logs)

		g.GET("/:id/members", h.GetMembers)
		g.PUT("/:id/members", h.SetMembers)

		g.GET("/:id/channels", h.GetChannels)
		g.PUT("/:id/channels", h.SetChannels)

		g.GET("/skills", h.ListSkills)
		g.POST("/skills/reload", h.ReloadSkills)

		g.POST("/:id/takeover", h.Takeover)
		g.POST("/:id/release", h.Release)

		// 沙箱串流（KasmVNC web 客户端经反向代理；iframe 用 ?token= 认证）
		g.Any("/:id/vnc/*path", h.Stream)
		g.Any("/:id/vnc", h.Stream)
	}
}

// currentUser 当前请求者
func currentUser(c *gin.Context) (userID, username, role string) {
	return c.GetString("user_id"), c.GetString("username"), c.GetString("role")
}

// fail 统一错误响应
func fail(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"success": false, "error": msg})
}

// ok 统一成功响应
func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// fillRole 填充请求者视角字段
func (h *Handler) fillRole(c *gin.Context, b *Bot) {
	userID, username, role := currentUser(c)
	b.MyRole = h.service.GetRole(userID, role, b.ID)
	if b.TakenOverBy == userID {
		b.TakenOverByName = username
	}
}

// List 列出所有 bot
func (h *Handler) List(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	bots, err := h.service.List(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	// 非平台管理员只看到自己是成员的 bot
	visible := make([]Bot, 0, len(bots))
	for i := range bots {
		b := bots[i]
		role := h.service.GetRole(userID, userRole, b.ID)
		if role == "" {
			continue
		}
		b.MyRole = role
		visible = append(visible, b)
	}
	ok(c, visible)
}

// Create 创建 bot
func (h *Handler) Create(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if userRole != "admin" {
		fail(c, 403, "仅平台管理员可创建 Bot")
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Model       string `json:"model"`
		ProviderID  string `json:"provider_id"`
		Image       string `json:"image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	b, err := h.service.Create(req.Name, req.Description, req.Model, req.ProviderID, req.Image)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	// 创建者默认成为该 bot 的 admin
	if err := h.service.SetMembers(b.ID, []BotMember{{UserID: userID, Role: RoleMemberAdmin}}); err != nil {
		_ = err // 成员设置失败不阻塞创建
	}
	h.fillRole(c, b)
	ok(c, b)
}

// Get 获取详情
func (h *Handler) Get(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	b, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, 404, err.Error())
		return
	}
	role := h.service.GetRole(userID, userRole, b.ID)
	if role == "" {
		fail(c, 403, "你不是该 Bot 的成员")
		return
	}
	b.MyRole = role
	// admin 角色可看成员列表
	if role == RoleMemberAdmin {
		if members, err := h.service.ListMembers(b.ID); err == nil {
			b.Members = members
		}
	}
	ok(c, b)
}

// Update 更新
func (h *Handler) Update(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	b, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, 404, err.Error())
		return
	}
	if role := h.service.GetRole(userID, userRole, b.ID); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Model       *string `json:"model"`
		ProviderID  *string `json:"provider_id"`
		Enabled     *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	b, err = h.service.Update(b.ID, req.Name, req.Description, req.Model, req.ProviderID, req.Enabled)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	h.fillRole(c, b)
	ok(c, b)
}

// Delete 删除
func (h *Handler) Delete(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限")
		return
	}
	removeData := c.Query("data") == "1" || c.Query("data") == "true"
	if err := h.service.Delete(c.Request.Context(), c.Param("id"), removeData); err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, nil)
}

// Start 启动沙箱
func (h *Handler) Start(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限")
		return
	}
	b, err := h.service.StartSandbox(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	h.fillRole(c, b)
	ok(c, b)
}

// Stop 停止沙箱
func (h *Handler) Stop(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限")
		return
	}
	b, err := h.service.StopSandbox(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	h.fillRole(c, b)
	ok(c, b)
}

// Restart 重启沙箱
func (h *Handler) Restart(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限")
		return
	}
	b, err := h.service.RestartSandbox(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	h.fillRole(c, b)
	ok(c, b)
}

// Exec 沙箱内执行脚本（bot 工具调用入口）
func (h *Handler) Exec(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限（工具调用仅在沙箱内执行）")
		return
	}
	var req ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	result, err := h.service.Exec(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, result)
}

// Logs 沙箱日志
func (h *Handler) Logs(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role == "" {
		fail(c, 403, "你不是该 Bot 的成员")
		return
	}
	logs, err := h.service.Logs(c.Request.Context(), c.Param("id"), 200)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, logs)
}

// GetMembers 成员列表
func (h *Handler) GetMembers(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限")
		return
	}
	members, err := h.service.ListMembers(c.Param("id"))
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, members)
}

// SetMembers 设置成员
func (h *Handler) SetMembers(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	if role := h.service.GetRole(userID, userRole, c.Param("id")); role != RoleMemberAdmin {
		fail(c, 403, "需要 Bot 管理权限")
		return
	}
	var req struct {
		Members []BotMember `json:"members" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	// 全删成员时兜底：至少保留一个 admin（含平台管理员语义）
	if err := h.service.SetMembers(c.Param("id"), req.Members); err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, nil)
}

// Takeover 接管
func (h *Handler) Takeover(c *gin.Context) {
	userID, username, userRole := currentUser(c)
	role := h.service.GetRole(userID, userRole, c.Param("id"))
	if role != RoleMemberAdmin {
		fail(c, 403, "只有 admin 角色可接管")
		return
	}
	if err := h.service.Takeover(c.Param("id"), userID, username); err != nil {
		fail(c, 409, err.Error())
		return
	}
	ok(c, nil)
}

// Release 交还
func (h *Handler) Release(c *gin.Context) {
	userID, _, _ := currentUser(c)
	if err := h.service.Release(c.Param("id"), userID); err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, nil)
}

// Stream 沙箱串流代理（预览/接管共用；viewer 角色无权访问）
func (h *Handler) Stream(c *gin.Context) {
	userID, _, userRole := currentUser(c)
	botID := c.Param("id")
	role := h.service.GetRole(userID, userRole, botID)
	if role == "" || role == RoleMemberViewer {
		c.JSON(403, gin.H{"error": "无串流权限（需要 member 或 admin 角色）"})
		return
	}
	h.proxy.Handle(c, botID)
}

// ---------- 渠道绑定（P3：网关消息路由） ----------

// GetChannels 查询渠道绑定
func (h *Handler) GetChannels(c *gin.Context) {
	channels, err := h.service.GetChannels(c.Param("id"))
	if err != nil {
		fail(c, 404, err.Error())
		return
	}
	ok(c, channels)
}

// SetChannels 全量设置渠道绑定
func (h *Handler) SetChannels(c *gin.Context) {
	var req struct {
		Channels []BotChannel `json:"channels" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}
	if _, err := h.service.UpdateChannels(c.Param("id"), req.Channels); err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, nil)
}

// ---------- 技能文件化（P3） ----------

// ListSkills 技能列表
func (h *Handler) ListSkills(c *gin.Context) {
	ok(c, h.skills.ListSkills())
}

// ReloadSkills 重新扫描技能目录
func (h *Handler) ReloadSkills(c *gin.Context) {
	if err := h.skills.Reload(); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, h.skills.ListSkills())
}
