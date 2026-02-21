// Package auth 提供登录失败速率限制
package auth

import (
	"sync"
	"time"
)

// LoginRateLimiter 登录速率限制器
// 跟踪每个 IP/用户名 的登录失败次数，超过阈值后锁定一段时间
type LoginRateLimiter struct {
	mu         sync.RWMutex
	attempts   map[string]*loginAttempt
	maxFails   int           // 最大允许失败次数
	lockoutDur time.Duration // 锁定时间
	windowDur  time.Duration // 统计窗口（失败次数在此窗口内累计）
}

type loginAttempt struct {
	failures  int
	firstFail time.Time
	lockedAt  time.Time
}

// NewLoginRateLimiter 创建登录速率限制器
// maxFails: 最大失败次数, lockout: 锁定时长, window: 统计窗口
func NewLoginRateLimiter(maxFails int, lockout, window time.Duration) *LoginRateLimiter {
	rl := &LoginRateLimiter{
		attempts:   make(map[string]*loginAttempt),
		maxFails:   maxFails,
		lockoutDur: lockout,
		windowDur:  window,
	}

	// 启动定期清理
	go rl.cleanup()

	return rl
}

// IsLocked 检查某个 key（IP 或用户名）是否被锁定
// 返回 (是否锁定, 剩余锁定秒数)
func (rl *LoginRateLimiter) IsLocked(key string) (bool, int) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	attempt, exists := rl.attempts[key]
	if !exists {
		return false, 0
	}

	// 检查是否在锁定期间
	if !attempt.lockedAt.IsZero() {
		remaining := time.Until(attempt.lockedAt.Add(rl.lockoutDur))
		if remaining > 0 {
			return true, int(remaining.Seconds())
		}
	}

	return false, 0
}

// RecordFailure 记录一次登录失败
// 返回 (是否触发锁定, 剩余可尝试次数)
func (rl *LoginRateLimiter) RecordFailure(key string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	attempt, exists := rl.attempts[key]

	if !exists {
		rl.attempts[key] = &loginAttempt{
			failures:  1,
			firstFail: now,
		}
		return false, rl.maxFails - 1
	}

	// 如果统计窗口已过期，重置
	if now.Sub(attempt.firstFail) > rl.windowDur {
		attempt.failures = 1
		attempt.firstFail = now
		attempt.lockedAt = time.Time{}
		return false, rl.maxFails - 1
	}

	// 如果已经被锁定且还在锁定期内，忽略
	if !attempt.lockedAt.IsZero() && now.Sub(attempt.lockedAt) < rl.lockoutDur {
		return true, 0
	}

	attempt.failures++

	if attempt.failures >= rl.maxFails {
		attempt.lockedAt = now
		return true, 0
	}

	return false, rl.maxFails - attempt.failures
}

// RecordSuccess 记录一次登录成功，清除失败记录
func (rl *LoginRateLimiter) RecordSuccess(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, key)
}

// cleanup 定期清理过期的记录
func (rl *LoginRateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, attempt := range rl.attempts {
			// 清理已过期的锁定和统计窗口
			if !attempt.lockedAt.IsZero() {
				if now.Sub(attempt.lockedAt) > rl.lockoutDur {
					delete(rl.attempts, key)
				}
			} else if now.Sub(attempt.firstFail) > rl.windowDur {
				delete(rl.attempts, key)
			}
		}
		rl.mu.Unlock()
	}
}
