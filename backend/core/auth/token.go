// Package auth 提供 JWT 令牌管理
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenManager JWT 令牌管理器
type TokenManager struct {
	config *Config
	secret []byte
}

// NewTokenManager 创建令牌管理器
func NewTokenManager(config *Config) *TokenManager {
	if config == nil {
		config = DefaultConfig()
	}
	return &TokenManager{
		config: config,
		secret: []byte(config.JWTSecret),
	}
}

// GenerateAccessToken 生成访问令牌
func (m *TokenManager) GenerateAccessToken(userID, username, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.AccessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefreshToken 生成刷新令牌
func (m *TokenManager) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.RefreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateTokenPair 生成令牌对
func (m *TokenManager) GenerateTokenPair(userID, username, role string) (*TokenPair, error) {
	accessToken, err := m.GenerateAccessToken(userID, username, role)
	if err != nil {
		return nil, err
	}

	refreshToken, err := m.GenerateRefreshToken(userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(m.config.AccessExpiry).Unix(),
		TokenType:    "Bearer",
	}, nil
}

// ParseAccessToken 解析访问令牌
func (m *TokenManager) ParseAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})

	if err != nil {
		if err == jwt.ErrTokenExpired {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ParseRefreshToken 解析刷新令牌
func (m *TokenManager) ParseRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})

	if err != nil {
		if err == jwt.ErrTokenExpired {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*RefreshClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ValidateToken 验证令牌
func (m *TokenManager) ValidateToken(tokenString string) bool {
	_, err := m.ParseAccessToken(tokenString)
	return err == nil
}

// RefreshTokenPair 使用刷新令牌生成新令牌对
func (m *TokenManager) RefreshTokenPair(refreshToken, username, role string) (*TokenPair, error) {
	claims, err := m.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	return m.GenerateTokenPair(claims.UserID, username, role)
}

// GenerateAPIKey 生成 API Key（长期令牌）
func (m *TokenManager) GenerateAPIKey(userID, username, role string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateTempToken 生成 2FA 临时令牌（5 分钟有效）
func (m *TokenManager) GenerateTempToken(userID, username, role string) (string, error) {
	now := time.Now()
	claims := TempClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		Purpose:  "2fa",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.config.Issuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseTempToken 解析 2FA 临时令牌
func (m *TokenManager) ParseTempToken(tokenString string) (*TempClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TempClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		if err == jwt.ErrTokenExpired {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if claims, ok := token.Claims.(*TempClaims); ok && token.Valid {
		if claims.Purpose != "2fa" {
			return nil, ErrInvalidToken
		}
		return claims, nil
	}
	return nil, ErrInvalidToken
}

// GetTokenExpiry 获取令牌过期时间
func (m *TokenManager) GetTokenExpiry(tokenString string) (time.Time, error) {
	claims, err := m.ParseAccessToken(tokenString)
	if err != nil {
		return time.Time{}, err
	}
	return claims.ExpiresAt.Time, nil
}

// GetUserIDFromToken 从令牌获取用户 ID
func (m *TokenManager) GetUserIDFromToken(tokenString string) (string, error) {
	claims, err := m.ParseAccessToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}
