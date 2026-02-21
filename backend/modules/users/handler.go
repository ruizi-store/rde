package users

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/core/auth"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	service      *Service
	tokenManager *auth.TokenManager
	logger       *zap.Logger
	rateLimiter  *auth.LoginRateLimiter
}

// NewHandler 创建处理器
func NewHandler(service *Service, tokenManager *auth.TokenManager, logger *zap.Logger) *Handler {
	return &Handler{
		service:      service,
		tokenManager: tokenManager,
		logger:       logger,
		// 5 次失败锁定 15 分钟，统计窗口 30 分钟
		rateLimiter: auth.NewLoginRateLimiter(5, 15*time.Minute, 30*time.Minute),
	}
}

// ----- 认证相关 -----

// Login 用户登录
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientIP := c.ClientIP()

	// 检查是否被 rate limit 锁定（同时检查 IP 和用户名）
	if locked, remaining := h.rateLimiter.IsLocked(clientIP); locked {
		h.logger.Warn("Login rate limited by IP", zap.String("ip", clientIP), zap.Int("remaining_seconds", remaining))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":            fmt.Sprintf("登录尝试过多，请 %d 分钟后重试", remaining/60+1),
			"retry_after_secs": remaining,
		})
		return
	}
	if locked, remaining := h.rateLimiter.IsLocked("user:" + req.Username); locked {
		h.logger.Warn("Login rate limited by username", zap.String("username", req.Username), zap.Int("remaining_seconds", remaining))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":            fmt.Sprintf("登录尝试过多，请 %d 分钟后重试", remaining/60+1),
			"retry_after_secs": remaining,
		})
		return
	}

	user, err := h.service.ValidatePassword(req.Username, req.Password)
	if err != nil {
		h.logger.Warn("Login failed", zap.String("username", req.Username), zap.Error(err))

		// 记录失败（IP 和用户名都记录）
		h.rateLimiter.RecordFailure(clientIP)
		ipLocked, attemptsLeft := h.rateLimiter.RecordFailure("user:" + req.Username)

		// 发布登录失败事件
		h.service.publishEvent("users.login.failed", map[string]string{
			"username": req.Username,
			"ip":       clientIP,
			"reason":   "invalid_credentials",
		})

		errMsg := "用户名或密码错误"
		if ipLocked {
			errMsg = "登录尝试过多，账户已临时锁定 15 分钟"
		} else if attemptsLeft > 0 && attemptsLeft <= 3 {
			errMsg = fmt.Sprintf("用户名或密码错误，还剩 %d 次尝试机会", attemptsLeft)
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": errMsg})
		return
	}

	// 登录成功，清除失败记录
	h.rateLimiter.RecordSuccess(clientIP)
	h.rateLimiter.RecordSuccess("user:" + req.Username)

	// 检查用户状态
	if user.Status != "active" {
		h.logger.Warn("Login failed - user disabled", zap.String("username", req.Username))
		// 发布登录失败事件（用户被禁用）
		h.service.publishEvent("users.login.failed", map[string]string{
			"username": req.Username,
			"ip":       clientIP,
			"reason":   "user_disabled",
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return
	}

	// 检查 TokenManager 是否可用
	if h.tokenManager == nil {
		h.logger.Error("TokenManager not available")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "认证服务不可用"})
		return
	}

	// 检查是否启用了 2FA
	if h.service.IsTotpEnabled(user.ID) {
		// 检查是否是受信任设备
		if req.DeviceToken != "" && h.service.IsTrustedDevice(user.ID, req.DeviceToken) {
			h.logger.Info("Trusted device login, skipping 2FA", zap.String("username", user.Username))
			// 受信任设备，跳过 2FA
		} else {
			// 生成临时令牌（5 分钟有效）
			tempToken, err := h.tokenManager.GenerateTempToken(user.ID, user.Username, user.Role)
			if err != nil {
				h.logger.Error("Failed to generate temp token", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "生成临时令牌失败"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"require_2fa": true,
					"temp_token":  tempToken,
				},
			})
			return
		}
	}

	// 生成 JWT Token
	tokenPair, err := h.tokenManager.GenerateTokenPair(user.ID, user.Username, user.Role)
	if err != nil {
		h.logger.Error("Failed to generate token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	// 更新最后登录时间
	h.service.UpdateLastLogin(user.ID)

	// 发布登录成功事件
	h.service.publishEvent("users.login.success", map[string]string{
		"user_id":  user.ID,
		"username": user.Username,
		"ip":       clientIP,
	})

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":          user,
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"expires_at":    tokenPair.ExpiresAt,
			"token_type":    tokenPair.TokenType,
		},
	})
}

// RefreshToken 刷新访问令牌
func (h *Handler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.tokenManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "认证服务不可用"})
		return
	}

	// 解析刷新令牌
	claims, err := h.tokenManager.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		h.logger.Warn("Invalid refresh token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的刷新令牌"})
		return
	}

	// 获取用户信息
	user, err := h.service.GetByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	// 检查用户状态
	if user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return
	}

	// 生成新的令牌对
	tokenPair, err := h.tokenManager.GenerateTokenPair(user.ID, user.Username, user.Role)
	if err != nil {
		h.logger.Error("Failed to generate token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"expires_at":    tokenPair.ExpiresAt,
			"token_type":    tokenPair.TokenType,
		},
	})
}

// Register 用户注册（已关闭，仅支持已登录用户通过 CreateUser 创建新用户）
func (h *Handler) Register(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "注册功能已关闭，请联系管理员创建账户"})
}

// ----- 两步验证（2FA） -----

// Verify2FA 验证 2FA 码完成登录（公开路由，通过 temp_token 鉴权）
func (h *Handler) Verify2FA(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TempToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少临时令牌"})
		return
	}

	// 解析临时令牌
	tempClaims, err := h.tokenManager.ParseTempToken(req.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "临时令牌无效或已过期，请重新登录"})
		return
	}

	// 验证 TOTP 码
	valid, err := h.service.Validate2FA(tempClaims.UserID, req.Code)
	if err != nil {
		h.logger.Error("2FA validation error", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证失败"})
		return
	}
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码错误"})
		return
	}

	// 验证通过，签发完整 token
	tokenPair, err := h.tokenManager.GenerateTokenPair(tempClaims.UserID, tempClaims.Username, tempClaims.Role)
	if err != nil {
		h.logger.Error("Failed to generate token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	// 获取用户信息
	user, _ := h.service.GetByID(tempClaims.UserID)
	h.service.UpdateLastLogin(tempClaims.UserID)

	// 如果用户选择记住此设备，生成设备信任令牌
	var deviceToken string
	if req.RememberDevice {
		deviceToken, err = h.service.AddTrustedDevice(tempClaims.UserID, c.GetHeader("User-Agent"), c.ClientIP())
		if err != nil {
			h.logger.Warn("Failed to add trusted device", zap.Error(err))
		}
	}

	h.service.publishEvent("users.login.success", map[string]string{
		"user_id":  tempClaims.UserID,
		"username": tempClaims.Username,
		"ip":       c.ClientIP(),
		"method":   "2fa",
	})

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":          user,
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"expires_at":    tokenPair.ExpiresAt,
			"token_type":    tokenPair.TokenType,
			"device_token":  deviceToken,
		},
	})
}

// Setup2FA 开始设置 2FA（需要已登录）
func (h *Handler) Setup2FA(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	result, err := h.service.Setup2FA(userID)
	if err != nil {
		h.logger.Error("Failed to setup 2FA", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置 2FA 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Enable2FA 验证并启用 2FA（需要已登录）
func (h *Handler) Enable2FA(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Enable2FA(userID, req.Code); err != nil {
		if err.Error() == "invalid verification code" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误，请重试"})
			return
		}
		h.logger.Error("Failed to enable 2FA", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启用 2FA 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "两步验证已启用"})
}

// Disable2FA 关闭 2FA（需要已登录）
func (h *Handler) Disable2FA(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	if err := h.service.Disable2FA(userID); err != nil {
		h.logger.Error("Failed to disable 2FA", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关闭 2FA 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "两步验证已关闭"})
}

// Get2FAStatus 获取 2FA 状态（需要已登录）
func (h *Handler) Get2FAStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	enabled := h.service.IsTotpEnabled(userID)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"enabled": enabled}})
}

// ----- 用户管理 -----

// ListUsers 获取用户列表
func (h *Handler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := h.service.ListUsers(page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户列表失败"})
		return
	}

	// 附加在线状态
	type UserWithOnline struct {
		User
		IsOnline bool `json:"is_online"`
	}
	result := make([]UserWithOnline, len(users))
	for i, u := range users {
		result[i] = UserWithOnline{
			User:     u,
			IsOnline: h.service.IsOnline(u.ID),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      result,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetUser 获取单个用户
func (h *Handler) GetUser(c *gin.Context) {
	id := c.Param("id")

	user, err := h.service.GetUserByID(id)
	if err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		h.logger.Error("Failed to get user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

// GetCurrentUser 获取当前用户
func (h *Handler) GetCurrentUser(c *gin.Context) {
	// TODO: 从 JWT 中获取用户 ID
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	user, err := h.service.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

// CreateUser 创建用户
func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 强制所有用户为 admin 角色
	req.Role = RoleAdmin

	if err := h.service.CreateUser(&req); err != nil {
		if err == ErrUserExists {
			c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
			return
		}
		h.logger.Error("Failed to create user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "创建成功"})
}

// UpdateUser 更新用户
func (h *Handler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateUser(id, &req); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		h.logger.Error("Failed to update user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteUser 删除用户
func (h *Handler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	// 禁止删除自己
	currentUserID := c.GetString("user_id")
	if id == currentUserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能删除自己的账户"})
		return
	}

	if err := h.service.DeleteUser(id); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		if err == ErrCannotDeleteAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "不能删除唯一的管理员"})
			return
		}
		h.logger.Error("Failed to delete user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ChangePassword 修改密码
func (h *Handler) ChangePassword(c *gin.Context) {
	id := c.Param("id")

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ChangePassword(id, req.OldPassword, req.NewPassword); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		if err == ErrInvalidPassword {
			c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
			return
		}
		h.logger.Error("Failed to change password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// ResetPassword 管理员重置密码（无需旧密码）
func (h *Handler) ResetPassword(c *gin.Context) {
	id := c.Param("id")

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ResetPassword(id, req.NewPassword); err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		h.logger.Error("Failed to reset password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

// UploadAvatar 上传用户头像
func (h *Handler) UploadAvatar(c *gin.Context) {
	id := c.Param("id")

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择头像文件"})
		return
	}

	// 限制文件大小 2MB
	if file.Size > 2*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像文件不能超过 2MB"})
		return
	}

	// 验证文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 jpg/png/webp 格式"})
		return
	}

	avatarURL, err := h.service.SaveAvatar(id, file, c)
	if err != nil {
		if err == ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		h.logger.Error("Failed to upload avatar", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "上传头像失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"avatar": avatarURL}})
}

// ----- 用户组管理 -----

// ListGroups 获取用户组列表
func (h *Handler) ListGroups(c *gin.Context) {
	groups, err := h.service.ListGroups()
	if err != nil {
		h.logger.Error("Failed to list groups", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户组列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": groups})
}

// CreateGroup 创建用户组
func (h *Handler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.service.CreateGroup(&req)
	if err != nil {
		if err == ErrGroupExists {
			c.JSON(http.StatusConflict, gin.H{"error": "用户组已存在"})
			return
		}
		h.logger.Error("Failed to create group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户组失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": group})
}

// UpdateGroup 更新用户组
func (h *Handler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateGroup(id, &req); err != nil {
		if err == ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
			return
		}
		h.logger.Error("Failed to update group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户组失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteGroup 删除用户组
func (h *Handler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteGroup(id); err != nil {
		if err == ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
			return
		}
		h.logger.Error("Failed to delete group", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户组失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
