// Package auth 提供 Gin 认证中间件
package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 上下文键
const (
	ContextKeyUserID   = "auth_user_id"
	ContextKeyUsername = "auth_username"
	ContextKeyRole     = "auth_role"
	ContextKeyClaims   = "auth_claims"
)

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	// Skipper 跳过认证的路径匹配函数
	Skipper func(c *gin.Context) bool

	// TokenExtractor 自定义令牌提取函数
	TokenExtractor func(c *gin.Context) string

	// ErrorHandler 自定义错误处理
	ErrorHandler func(c *gin.Context, err error)

	// SuccessHandler 认证成功后的回调
	SuccessHandler func(c *gin.Context, claims *Claims)
}

// DefaultMiddlewareConfig 默认中间件配置
func DefaultMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		Skipper:        nil,
		TokenExtractor: nil,
		ErrorHandler:   nil,
		SuccessHandler: nil,
	}
}

// Middleware 创建 JWT 认证中间件
func Middleware(tokenManager *TokenManager) gin.HandlerFunc {
	return MiddlewareWithConfig(tokenManager, DefaultMiddlewareConfig())
}

// MiddlewareWithConfig 创建可配置的 JWT 认证中间件
func MiddlewareWithConfig(tokenManager *TokenManager, config *MiddlewareConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultMiddlewareConfig()
	}

	return func(c *gin.Context) {
		// 检查是否跳过
		if config.Skipper != nil && config.Skipper(c) {
			c.Next()
			return
		}

		// 提取令牌
		var token string
		if config.TokenExtractor != nil {
			token = config.TokenExtractor(c)
		} else {
			token = ExtractToken(c)
		}

		if token == "" {
			handleError(c, config, ErrUnauthorized)
			return
		}

		// 解析令牌
		claims, err := tokenManager.ParseAccessToken(token)
		if err != nil {
			handleError(c, config, err)
			return
		}

		// 存入上下文
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)
		c.Set(ContextKeyClaims, claims)
		// 兼容旧代码：同时设置简短的键名
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		// 成功回调
		if config.SuccessHandler != nil {
			config.SuccessHandler(c, claims)
		}

		c.Next()
	}
}

// OptionalMiddleware 可选认证中间件（不强制要求令牌）
func OptionalMiddleware(tokenManager *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ExtractToken(c)
		if token != "" {
			claims, err := tokenManager.ParseAccessToken(token)
			if err == nil {
				c.Set(ContextKeyUserID, claims.UserID)
				c.Set(ContextKeyUsername, claims.Username)
				c.Set(ContextKeyRole, claims.Role)
				c.Set(ContextKeyClaims, claims)
			}
		}
		c.Next()
	}
}

// RequireRole 角色验证中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetRole(c)
		if role == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": http.StatusForbidden,
				"message": "access denied",
			})
			return
		}

		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": http.StatusForbidden,
			"message": "insufficient permissions",
		})
	}
}

// RequireAdmin 管理员权限中间件
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}

// handleError 处理认证错误
func handleError(c *gin.Context, config *MiddlewareConfig, err error) {
	if config.ErrorHandler != nil {
		config.ErrorHandler(c, err)
		return
	}

	status := http.StatusUnauthorized
	message := "unauthorized"

	switch err {
	case ErrExpiredToken:
		message = "token expired"
	case ErrInvalidToken:
		message = "invalid token"
	case ErrForbidden:
		status = http.StatusForbidden
		message = "forbidden"
	}

	c.AbortWithStatusJSON(status, gin.H{
		"success": status,
		"message": message,
	})
}

// ExtractToken 从请求中提取令牌
func ExtractToken(c *gin.Context) string {
	// 1. 从 Authorization 头提取
	auth := c.GetHeader("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		return auth
	}

	// 2. 从查询参数提取
	if token := c.Query("token"); token != "" {
		return token
	}

	// 3. 从 Cookie 提取
	if token, err := c.Cookie("token"); err == nil && token != "" {
		return token
	}

	return ""
}

// GetUserID 从上下文获取用户 ID
func GetUserID(c *gin.Context) uint {
	if id, exists := c.Get(ContextKeyUserID); exists {
		if uid, ok := id.(uint); ok {
			return uid
		}
	}
	return 0
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(ContextKeyUsername); exists {
		if u, ok := username.(string); ok {
			return u
		}
	}
	return ""
}

// GetRole 从上下文获取角色
func GetRole(c *gin.Context) string {
	if role, exists := c.Get(ContextKeyRole); exists {
		if r, ok := role.(string); ok {
			return r
		}
	}
	return ""
}

// GetClaims 从上下文获取完整 Claims
func GetClaims(c *gin.Context) *Claims {
	if claims, exists := c.Get(ContextKeyClaims); exists {
		if c, ok := claims.(*Claims); ok {
			return c
		}
	}
	return nil
}

// IsAuthenticated 检查是否已认证
func IsAuthenticated(c *gin.Context) bool {
	return GetUserID(c) > 0
}

// IsAdmin 检查是否为管理员
func IsAdmin(c *gin.Context) bool {
	return GetRole(c) == "admin"
}

// SkipPaths 创建路径跳过函数
func SkipPaths(paths ...string) func(c *gin.Context) bool {
	return func(c *gin.Context) bool {
		path := c.Request.URL.Path
		for _, p := range paths {
			if path == p || strings.HasPrefix(path, p) {
				return true
			}
		}
		return false
	}
}

// SkipPathsAndMethods 创建路径和方法跳过函数
func SkipPathsAndMethods(skipPaths []string, skipMethods []string) func(c *gin.Context) bool {
	return func(c *gin.Context) bool {
		// 检查方法
		method := c.Request.Method
		for _, m := range skipMethods {
			if method == m {
				return true
			}
		}

		// 检查路径
		path := c.Request.URL.Path
		for _, p := range skipPaths {
			if path == p || strings.HasPrefix(path, p) {
				return true
			}
		}

		return false
	}
}
