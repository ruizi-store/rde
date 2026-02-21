package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

// GetStatus 获取初始化状态
// GET /api/v1/setup/status
func (h *Handler) GetStatus(c *gin.Context) {
	status, err := h.service.GetStatus()
	if err != nil {
		h.logger.Error("Failed to get setup status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取初始化状态失败"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// CheckSystem 执行系统检查
// GET /api/v1/setup/check
func (h *Handler) CheckSystem(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	result, err := h.service.CheckSystem()
	if err != nil {
		h.logger.Error("Failed to check system", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "系统检查失败"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CompleteStep1 完成系统检查步骤
// POST /api/v1/setup/check/complete
func (h *Handler) CompleteStep1(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	if err := h.service.CompleteStep1(); err != nil {
		h.logger.Error("Failed to complete step 1", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "系统检查完成"})
}

// InstallDeps 安装缺失的依赖（SSE 流式响应）
// POST /api/v1/setup/install-deps
func (h *Handler) InstallDeps(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req InstallDepsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	progressChan := make(chan map[string]interface{}, 10)

	go func() {
		h.service.InstallDependencies(req.Packages, progressChan)
	}()

	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-progressChan; ok {
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
			return true
		}

		// 发送完成事件
		fmt.Fprintf(w, "event: done\ndata: {\"success\": true}\n\n")
		return false
	})
}

// SetLocale 设置语言和时区
// POST /api/v1/setup/locale
func (h *Handler) SetLocale(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req LocaleSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SetLocale(&req); err != nil {
		h.logger.Error("Failed to set locale", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置语言时区失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "语言时区设置成功"})
}

// CreateAdmin 创建管理员
// POST /api/v1/setup/user
func (h *Handler) CreateAdmin(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req SetupUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	twoFactorSetup, err := h.service.CreateAdmin(&req)
	if err != nil {
		if err == ErrUserExists {
			c.JSON(http.StatusConflict, gin.H{"error": "管理员已存在"})
			return
		}
		if err == ErrReservedUsername {
			c.JSON(http.StatusBadRequest, gin.H{"error": "用户名为保留名称"})
			return
		}
		h.logger.Error("Failed to create admin", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建管理员失败"})
		return
	}

	response := gin.H{"message": "管理员创建成功"}
	if twoFactorSetup != nil {
		response["two_factor"] = twoFactorSetup
	}

	c.JSON(http.StatusCreated, response)
}

// Verify2FA 验证 2FA
// POST /api/v1/setup/user/verify-2fa
func (h *Handler) Verify2FA(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从 context 获取用户 ID（需要在创建用户后传递）
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户 ID"})
		return
	}

	if err := h.service.Verify2FA(userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "2FA 验证成功"})
}

// ConfigureSecurity 配置安全选项
// POST /api/v1/setup/security
func (h *Handler) ConfigureSecurity(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req SecurityConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.ConfigureSecurity(&req)
	if err != nil {
		h.logger.Error("Failed to configure security", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "安全配置失败"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetDrives 获取检测到的硬盘
// GET /api/v1/setup/drives
func (h *Handler) GetDrives(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	drives, err := h.service.GetDrives()
	if err != nil {
		h.logger.Error("Failed to get drives", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取硬盘列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": drives})
}

// ConfigureStorage 配置存储
// POST /api/v1/setup/storage
func (h *Handler) ConfigureStorage(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req StorageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ConfigureStorage(&req); err != nil {
		h.logger.Error("Failed to configure storage", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存储配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "存储配置成功"})
}

// SkipStorage 跳过存储配置
// POST /api/v1/setup/storage/skip
func (h *Handler) SkipStorage(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	if err := h.service.SkipStorageConfig(); err != nil {
		h.logger.Error("Failed to skip storage config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "跳过存储配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已跳过存储配置"})
}

// GetAvailableDisks 获取可用于创建存储池的硬盘
// GET /api/v1/setup/storage/available-disks
func (h *Handler) GetAvailableDisks(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	disks, err := h.service.GetAvailableDisks()
	if err != nil {
		h.logger.Error("Failed to get available disks", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取可用硬盘失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": disks})
}

// ConfigureNetwork 配置网络
// POST /api/v1/setup/network
func (h *Handler) ConfigureNetwork(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req NetworkConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ConfigureNetwork(&req); err != nil {
		h.logger.Error("Failed to configure network", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "网络配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "网络配置成功"})
}

// SkipNetwork 跳过网络配置
// POST /api/v1/setup/network/skip
func (h *Handler) SkipNetwork(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	if err := h.service.SkipNetworkConfig(); err != nil {
		h.logger.Error("Failed to skip network config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "跳过网络配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已跳过网络配置"})
}

// ----- Step 6: 功能选择 -----

// GetFeatures 获取可选功能列表
// GET /api/v1/setup/features
func (h *Handler) GetFeatures(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	options, err := h.service.GetFeatureOptions()
	if err != nil {
		h.logger.Error("Failed to get feature options", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取功能列表失败"})
		return
	}

	c.JSON(http.StatusOK, options)
}

// SaveFeatures 保存功能选择
// POST /api/v1/setup/features
func (h *Handler) SaveFeatures(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	var req FeatureSelection
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SaveFeatureSelection(&req); err != nil {
		h.logger.Error("Failed to save feature selection", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "功能选择已保存"})
}

// SkipFeatures 跳过功能选择
// POST /api/v1/setup/features/skip
func (h *Handler) SkipFeatures(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	if err := h.service.SkipFeatureSelection(); err != nil {
		h.logger.Error("Failed to skip feature selection", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "跳过功能选择失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已使用推荐配置"})
}

// ----- Step 7: 完成 -----

// Complete 完成初始化
// POST /api/v1/setup/complete
func (h *Handler) Complete(c *gin.Context) {
	if h.service.IsCompleted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "初始化已完成"})
		return
	}

	response, err := h.service.Complete()
	if err != nil {
		h.logger.Error("Failed to complete setup", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// FactoryReset 恢复出厂设置
// POST /api/v1/setup/factory-reset
// 需要 JWT 认证，且必须是管理员，还需要再次输入密码确认
func (h *Handler) FactoryReset(c *gin.Context) {
	// 从 JWT 上下文获取用户信息
	userID, exists := c.Get("auth_user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	role, _ := c.Get("auth_role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有管理员可以执行此操作"})
		return
	}

	username, _ := c.Get("auth_username")

	var req FactoryResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证当前用户密码（二次确认）
	if err := h.service.ValidateUserPassword(userID.(string), req.Password); err != nil {
		h.logger.Warn("Factory reset password validation failed", zap.Any("user_id", userID))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	h.logger.Info("Factory reset requested",
		zap.Any("user_id", userID),
		zap.Any("username", username))

	response, err := h.service.FactoryReset(&req)
	if err != nil {
		h.logger.Error("Factory reset failed", zap.Error(err))

		var statusCode int
		var message string
		switch err {
		case ErrInvalidConfirmText:
			statusCode = http.StatusBadRequest
			message = "确认文本错误，请输入 RESET"
		default:
			statusCode = http.StatusInternalServerError
			message = "恢复出厂设置失败"
		}

		c.JSON(statusCode, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ==================== 模块设置 API ====================

// GetModuleSettings 获取所有模块设置
// GET /api/v1/settings/modules
func (h *Handler) GetModuleSettings(c *gin.Context) {
	settings, err := h.service.GetAllModuleSettings()
	if err != nil {
		h.logger.Error("Failed to get module settings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取模块设置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": settings})
}

// UpdateModuleSetting 更新模块设置
// PUT /api/v1/settings/modules/:id
func (h *Handler) UpdateModuleSetting(c *gin.Context) {
	moduleID := c.Param("id")
	if moduleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模块 ID 不能为空"})
		return
	}

	var req struct {
		Enabled bool                   `json:"enabled"`
		Config  map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	setting, err := h.service.UpdateModuleSetting(moduleID, req.Enabled, req.Config)
	if err != nil {
		h.logger.Error("Failed to update module setting", zap.Error(err), zap.String("module_id", moduleID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模块设置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    setting,
		"message": "设置已保存，重启后生效",
	})
}
