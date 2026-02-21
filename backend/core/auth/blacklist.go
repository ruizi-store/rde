// Package auth 提供令牌黑名单持久化存储
package auth

import (
	"time"

	"gorm.io/gorm"
)

// BlacklistEntry 黑名单条目（数据库模型）
type BlacklistEntry struct {
	ID        uint      `gorm:"primaryKey"`
	Token     string    `gorm:"uniqueIndex;size:512"`
	ExpiresAt int64     `gorm:"index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (BlacklistEntry) TableName() string {
	return "auth_token_blacklist"
}

// PersistentBlacklist 持久化黑名单（基于 GORM/SQLite）
type PersistentBlacklist struct {
	db *gorm.DB
}

// NewPersistentBlacklist 创建持久化黑名单
func NewPersistentBlacklist(db *gorm.DB) *PersistentBlacklist {
	// 自动创建表
	db.AutoMigrate(&BlacklistEntry{})

	bl := &PersistentBlacklist{db: db}

	// 启动清理协程
	go bl.cleanup()

	return bl
}

// Add 添加令牌到黑名单
func (bl *PersistentBlacklist) Add(token string, expiresAt int64) error {
	entry := BlacklistEntry{
		Token:     token,
		ExpiresAt: expiresAt,
	}
	return bl.db.Create(&entry).Error
}

// Contains 检查令牌是否在黑名单中
func (bl *PersistentBlacklist) Contains(token string) bool {
	var count int64
	bl.db.Model(&BlacklistEntry{}).Where("token = ?", token).Count(&count)
	return count > 0
}

// Remove 从黑名单移除令牌
func (bl *PersistentBlacklist) Remove(token string) error {
	return bl.db.Where("token = ?", token).Delete(&BlacklistEntry{}).Error
}

// Cleanup 清理过期的黑名单条目
func (bl *PersistentBlacklist) CleanupExpired() {
	bl.db.Where("expires_at < ?", time.Now().Unix()).Delete(&BlacklistEntry{})
}

// cleanup 定期清理过期条目
func (bl *PersistentBlacklist) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		bl.CleanupExpired()
	}
}
