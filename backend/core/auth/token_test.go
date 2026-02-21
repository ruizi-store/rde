package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenManager_GenerateAndParse(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	// 生成访问令牌
	token, err := tm.GenerateAccessToken("1", "testuser", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// 解析令牌
	claims, err := tm.ParseAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, "1", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "admin", claims.Role)
}

func TestTokenManager_GenerateTokenPair(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	pair, err := tm.GenerateTokenPair("1", "testuser", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, "Bearer", pair.TokenType)
	assert.Greater(t, pair.ExpiresAt, time.Now().Unix())
}

func TestTokenManager_RefreshToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	// 生成初始令牌对
	pair, err := tm.GenerateTokenPair("1", "testuser", "admin")
	require.NoError(t, err)

	// 等待一小段时间确保新令牌时间不同
	time.Sleep(time.Millisecond * 10)

	// 使用刷新令牌获取新令牌
	newPair, err := tm.RefreshTokenPair(pair.RefreshToken, "testuser", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, newPair.AccessToken)

	// 验证新令牌可以正常解析
	claims, err := tm.ParseAccessToken(newPair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "1", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
}

func TestTokenManager_InvalidToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	// 解析无效令牌
	_, err := tm.ParseAccessToken("invalid-token")
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidToken, err)
}

func TestTokenManager_ExpiredToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  -time.Hour, // 过期的令牌
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	token, err := tm.GenerateAccessToken("1", "testuser", "admin")
	require.NoError(t, err)

	_, err = tm.ParseAccessToken(token)
	assert.Error(t, err)
}

func TestTokenManager_WrongSecret(t *testing.T) {
	config1 := &Config{
		JWTSecret:     "secret-1",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	config2 := &Config{
		JWTSecret:     "secret-2",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm1 := NewTokenManager(config1)
	tm2 := NewTokenManager(config2)

	// 用 tm1 生成令牌
	token, err := tm1.GenerateAccessToken("1", "testuser", "admin")
	require.NoError(t, err)

	// 用 tm2 解析应该失败
	_, err = tm2.ParseAccessToken(token)
	assert.Error(t, err)
}

func TestTokenManager_ValidateToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	token, _ := tm.GenerateAccessToken("1", "testuser", "admin")

	assert.True(t, tm.ValidateToken(token))
	assert.False(t, tm.ValidateToken("invalid"))
}

func TestTokenManager_GetUserIDFromToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	token, _ := tm.GenerateAccessToken("42", "testuser", "admin")

	userID, err := tm.GetUserIDFromToken(token)
	require.NoError(t, err)
	assert.Equal(t, "42", userID)
}

func TestTokenManager_GenerateAPIKey(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)

	// 生成 365 天有效的 API Key
	apiKey, err := tm.GenerateAPIKey("1", "apiuser", "api", 365*24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, apiKey)

	// 验证 API Key
	claims, err := tm.ParseAccessToken(apiKey)
	require.NoError(t, err)
	assert.Equal(t, "1", claims.UserID)
	assert.Equal(t, "apiuser", claims.Username)
}

func TestHashPassword(t *testing.T) {
	password := "mypassword123"

	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// 验证正确密码
	assert.True(t, CheckPassword(password, hash))

	// 验证错误密码
	assert.False(t, CheckPassword("wrongpassword", hash))
}

func TestService_Blacklist(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	service := NewService(config, nil)

	token, _ := service.GetTokenManager().GenerateAccessToken("1", "testuser", "admin")

	// 令牌应该有效
	assert.False(t, service.IsTokenBlacklisted(token))

	// 登出（加入黑名单）
	err := service.Logout(nil, token)
	require.NoError(t, err)

	// 令牌应该在黑名单中
	assert.True(t, service.IsTokenBlacklisted(token))
}

func TestService_CleanupBlacklist(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret-key",
		AccessExpiry:  time.Millisecond, // 很短的过期时间
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	service := NewService(config, nil)

	token, _ := service.GetTokenManager().GenerateAccessToken("1", "testuser", "admin")

	// 等待令牌过期
	time.Sleep(10 * time.Millisecond)

	// 手动添加到黑名单（模拟过期令牌）
	service.blacklistMu.Lock()
	service.blacklist[token] = time.Now().Add(-time.Hour).Unix() // 已过期
	service.blacklistMu.Unlock()

	// 清理
	service.CleanupBlacklist()

	// 应该已被清理
	assert.False(t, service.IsTokenBlacklisted(token))
}
