// Package auth — 认证模式（免登录 / 本地账号）
//
// 容器唯一交付形态下默认免登录（none）：起容器即用，面向内网/反代后的
// 自托管场景。需要账号体系时显式设置 RDE_AUTH_MODE=local（登录页与
// 安装向导恢复）。IM 网关外部回调（webhook/telegram/wecom）不受模式
// 影响，始终使用各自的 API Key / secret_token / 签名鉴权。
package auth

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// 认证模式
const (
	AuthModeNone  = "none"  // 免登录单用户模式（默认）
	AuthModeLocal = "local" // 本地账号 + JWT 登录
)

// 免登录模式的固定身份（审计字段、RequireAdmin 等照常工作）
const (
	NoneUserID   = "local-admin"
	NoneUsername = "admin"
)

// ResolveAuthMode 解析认证模式（环境变量 RDE_AUTH_MODE）
// 未设置或值非法时返回默认 none。
func ResolveAuthMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("RDE_AUTH_MODE")))
	switch mode {
	case AuthModeLocal:
		return AuthModeLocal
	default:
		return AuthModeNone
	}
}

// IsAuthNone 当前是否免登录模式
func IsAuthNone() bool {
	return ResolveAuthMode() == AuthModeNone
}

// NoneMiddleware 免登录模式中间件：注入固定管理员身份，不做 token 校验。
// 应用在 /api/v1 组（bootstrap 在 none 模式下用它替换 JWT 中间件）。
func NoneMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextKeyUserID, NoneUserID)
		c.Set(ContextKeyUsername, NoneUsername)
		c.Set(ContextKeyRole, "admin")
		// 兼容旧代码的简短键名
		c.Set("user_id", NoneUserID)
		c.Set("username", NoneUsername)
		c.Set("role", "admin")
		c.Next()
	}
}
