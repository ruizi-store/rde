// Package auth 提供认证服务
package auth

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// UserProvider 用户信息提供者接口
// 由 users 模块实现此接口
type UserProvider interface {
	// GetUserByID 根据 ID 获取用户
	GetUserByID(ctx context.Context, id string) (*UserInfo, error)

	// GetUserByUsername 根据用户名获取用户
	GetUserByUsername(ctx context.Context, username string) (*UserInfo, string, error) // 返回用户信息和密码哈希

	// UpdateLastLogin 更新最后登录时间
	UpdateLastLogin(ctx context.Context, id string) error
}

// Service 认证服务
type Service struct {
	config       *Config
	tokenManager *TokenManager
	userProvider UserProvider
	logger       *zap.Logger

	// 令牌黑名单（内存回退 + 可选持久化）
	blacklist           map[string]int64
	blacklistMu         sync.RWMutex
	persistentBlacklist *PersistentBlacklist // 可选，设置后使用持久化
}

// NewService 创建认证服务
func NewService(config *Config, logger *zap.Logger) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		config:       config,
		tokenManager: NewTokenManager(config),
		logger:       logger,
		blacklist:    make(map[string]int64),
	}
}

// SetPersistentBlacklist 设置持久化黑名单（可选）
func (s *Service) SetPersistentBlacklist(bl *PersistentBlacklist) {
	s.persistentBlacklist = bl
	s.logger.Info("Persistent token blacklist enabled")
}

// SetUserProvider 设置用户提供者
func (s *Service) SetUserProvider(provider UserProvider) {
	s.userProvider = provider
}

// GetTokenManager 获取令牌管理器
func (s *Service) GetTokenManager() *TokenManager {
	return s.tokenManager
}

// GetConfig 获取配置
func (s *Service) GetConfig() *Config {
	return s.config
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, username, password string) (*TokenPair, *UserInfo, error) {
	if s.userProvider == nil {
		return nil, nil, ErrUserNotFound
	}

	// 获取用户
	user, passwordHash, err := s.userProvider.GetUserByUsername(ctx, username)
	if err != nil {
		s.logger.Debug("user not found", zap.String("username", username))
		return nil, nil, ErrInvalidCredentials
	}

	// 检查用户状态
	if user.Status != "active" {
		return nil, nil, ErrUserDisabled
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		s.logger.Debug("password mismatch", zap.String("username", username))
		return nil, nil, ErrInvalidCredentials
	}

	// 生成令牌
	tokenPair, err := s.tokenManager.GenerateTokenPair(user.ID, user.Username, user.Role)
	if err != nil {
		s.logger.Error("failed to generate token", zap.Error(err))
		return nil, nil, err
	}

	// 更新最后登录时间
	go func() {
		if err := s.userProvider.UpdateLastLogin(context.Background(), user.ID); err != nil {
			s.logger.Warn("failed to update last login", zap.Error(err))
		}
	}()

	s.logger.Info("user logged in", zap.String("username", username), zap.String("user_id", user.ID))

	return tokenPair, user, nil
}

// Logout 用户登出（将令牌加入黑名单）
func (s *Service) Logout(ctx context.Context, token string) error {
	claims, err := s.tokenManager.ParseAccessToken(token)
	if err != nil {
		return err
	}

	// 将令牌加入黑名单（优先使用持久化）
	if s.persistentBlacklist != nil {
		if err := s.persistentBlacklist.Add(token, claims.ExpiresAt.Unix()); err != nil {
			s.logger.Warn("Failed to persist blacklist entry, falling back to memory", zap.Error(err))
			s.blacklistMu.Lock()
			s.blacklist[token] = claims.ExpiresAt.Unix()
			s.blacklistMu.Unlock()
		}
	} else {
		s.blacklistMu.Lock()
		s.blacklist[token] = claims.ExpiresAt.Unix()
		s.blacklistMu.Unlock()
	}

	s.logger.Info("user logged out", zap.String("user_id", claims.UserID))

	return nil
}

// RefreshToken 刷新令牌
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// 解析刷新令牌
	claims, err := s.tokenManager.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	if s.userProvider == nil {
		return nil, ErrUserNotFound
	}

	// 获取用户最新信息
	user, err := s.userProvider.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if user.Status != "active" {
		return nil, ErrUserDisabled
	}

	// 生成新令牌对
	return s.tokenManager.GenerateTokenPair(user.ID, user.Username, user.Role)
}

// ValidateToken 验证令牌
func (s *Service) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	// 检查黑名单（优先持久化，再回退内存）
	if s.persistentBlacklist != nil {
		if s.persistentBlacklist.Contains(token) {
			return nil, ErrInvalidToken
		}
	}

	s.blacklistMu.RLock()
	_, blacklisted := s.blacklist[token]
	s.blacklistMu.RUnlock()

	if blacklisted {
		return nil, ErrInvalidToken
	}

	return s.tokenManager.ParseAccessToken(token)
}

// IsTokenBlacklisted 检查令牌是否在黑名单中
func (s *Service) IsTokenBlacklisted(token string) bool {
	s.blacklistMu.RLock()
	defer s.blacklistMu.RUnlock()
	_, exists := s.blacklist[token]
	return exists
}

// CleanupBlacklist 清理过期的黑名单条目
func (s *Service) CleanupBlacklist() {
	s.blacklistMu.Lock()
	defer s.blacklistMu.Unlock()

	now := time.Now().Unix()
	for token, expiry := range s.blacklist {
		if expiry < now {
			delete(s.blacklist, token)
		}
	}
}

// StartBlacklistCleanup 启动黑名单清理定时任务
func (s *Service) StartBlacklistCleanup(interval time.Duration) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.CleanupBlacklist()
			case <-stop:
				return
			}
		}
	}()
	return stop
}

// HashPassword 哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateAPIKey 生成 API Key
func (s *Service) GenerateAPIKey(ctx context.Context, userID, username, role string, expiryDays int) (string, error) {
	expiry := time.Duration(expiryDays) * 24 * time.Hour
	return s.tokenManager.GenerateAPIKey(userID, username, role, expiry)
}

// GetUserFromToken 从令牌获取用户信息
func (s *Service) GetUserFromToken(ctx context.Context, token string) (*UserInfo, error) {
	claims, err := s.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if s.userProvider != nil {
		return s.userProvider.GetUserByID(ctx, claims.UserID)
	}

	// 如果没有用户提供者，返回 Claims 中的信息
	return &UserInfo{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}
