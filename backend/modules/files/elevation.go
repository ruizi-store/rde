package files

import (
	"sync"
	"time"
)

const (
	// ElevationDuration 管理员提权持续时间
	ElevationDuration = 5 * time.Minute
)

// ElevationSession 提权会话信息
type ElevationSession struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ElevationManager 管理员提权会话管理器
type ElevationManager struct {
	mu       sync.RWMutex
	sessions map[string]*ElevationSession // key: userID
}

// NewElevationManager 创建提权会话管理器
func NewElevationManager() *ElevationManager {
	em := &ElevationManager{
		sessions: make(map[string]*ElevationSession),
	}

	// 启动定期清理过期会话的 goroutine
	go em.cleanupLoop()

	return em
}

// Elevate 创建提权会话
func (em *ElevationManager) Elevate(userID, username string) *ElevationSession {
	em.mu.Lock()
	defer em.mu.Unlock()

	session := &ElevationSession{
		UserID:    userID,
		Username:  username,
		ExpiresAt: time.Now().Add(ElevationDuration),
	}
	em.sessions[userID] = session
	return session
}

// Revoke 撤销提权
func (em *ElevationManager) Revoke(userID string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	delete(em.sessions, userID)
}

// IsElevated 检查用户是否处于提权状态
func (em *ElevationManager) IsElevated(userID string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()

	session, exists := em.sessions[userID]
	if !exists {
		return false
	}

	if time.Now().After(session.ExpiresAt) {
		// 已过期，异步清理
		go em.Revoke(userID)
		return false
	}

	return true
}

// GetStatus 获取提权状态
func (em *ElevationManager) GetStatus(userID string) *ElevationSession {
	em.mu.RLock()
	defer em.mu.RUnlock()

	session, exists := em.sessions[userID]
	if !exists {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		go em.Revoke(userID)
		return nil
	}

	return session
}

// cleanupLoop 定期清理过期会话
func (em *ElevationManager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		em.mu.Lock()
		now := time.Now()
		for uid, session := range em.sessions {
			if now.After(session.ExpiresAt) {
				delete(em.sessions, uid)
			}
		}
		em.mu.Unlock()
	}
}
