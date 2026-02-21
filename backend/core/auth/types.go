// Package auth 提供认证服务核心
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token expired")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserDisabled      = errors.New("user is disabled")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
)

// Claims JWT Claims 结构
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// RefreshClaims 刷新令牌 Claims
type RefreshClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// TempClaims 2FA 临时令牌 Claims（密码已验证，等待 TOTP）
type TempClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Purpose  string `json:"purpose"` // 固定为 "2fa"
	jwt.RegisteredClaims
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	TokenType    string `json:"token_type"`
}

// UserInfo 用户基本信息（用于认证上下文）
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

// Config 认证配置
type Config struct {
	JWTSecret       string        `json:"jwt_secret"`
	AccessExpiry    time.Duration `json:"access_expiry"`
	RefreshExpiry   time.Duration `json:"refresh_expiry"`
	Issuer          string        `json:"issuer"`
	TokenLookup     string        `json:"token_lookup"` // header:Authorization, query:token, cookie:token
	TokenHeaderName string        `json:"token_header_name"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		JWTSecret:       "rde-default-secret-change-me",
		AccessExpiry:    24 * time.Hour,
		RefreshExpiry:   7 * 24 * time.Hour,
		Issuer:          "rde",
		TokenLookup:     "header:Authorization,query:token,cookie:token",
		TokenHeaderName: "Authorization",
	}
}
