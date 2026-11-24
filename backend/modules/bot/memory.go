// Package bot — Bot 长期记忆（P4）
//
// 每个 Bot 一份 SQLite 记忆库（bots_memories 表），由 LLM 通过工具自主管理：
// save_memory（同 key 覆盖）/ search_memory（关键词 LIKE 检索，空关键词返回最近条目）。
// 人也可以经 REST 查看/删除，便于审计 Bot 记住了什么。
package bot

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BotMemory 记忆条目
type BotMemory struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	BotID     string    `json:"bot_id" gorm:"uniqueIndex:idx_bot_mem_key;size:36;not null"`
	Key       string    `json:"key" gorm:"uniqueIndex:idx_bot_mem_key;size:128;not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (BotMemory) TableName() string { return "bots_memories" }

// MemoryService 记忆存取（实现 ai.MemoryProvider）
type MemoryService struct {
	db *gorm.DB
}

// NewMemoryService 创建记忆服务
func NewMemoryService(db *gorm.DB) *MemoryService {
	return &MemoryService{db: db}
}

// SaveMemory 保存/覆盖一条记忆（同 botID+key 覆盖）
func (m *MemoryService) SaveMemory(botID, key, content string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("记忆 key 不能为空")
	}
	if len(key) > 128 {
		return fmt.Errorf("记忆 key 过长（上限 128 字符）")
	}
	if content == "" {
		return fmt.Errorf("记忆内容不能为空")
	}

	var existing BotMemory
	err := m.db.Where("bot_id = ? AND key = ?", botID, key).First(&existing).Error
	if err == nil {
		return m.db.Model(&existing).Updates(map[string]interface{}{
			"content":    content,
			"updated_at": time.Now(),
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return m.db.Create(&BotMemory{BotID: botID, Key: key, Content: content}).Error
}

// SearchMemory 按关键词检索（OR 匹配 key 与 content；空关键词返回最近条目）
func (m *MemoryService) SearchMemory(botID, query string, limit int) ([]BotMemory, error) {
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	q := m.db.Where("bot_id = ?", botID)
	if kw := strings.TrimSpace(query); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("key LIKE ? OR content LIKE ?", like, like)
	} else {
		q = q.Order("updated_at DESC")
	}
	var memories []BotMemory
	if err := q.Order("updated_at DESC").Limit(limit).Find(&memories).Error; err != nil {
		return nil, err
	}
	return memories, nil
}

// DeleteMemory 删除一条记忆
func (m *MemoryService) DeleteMemory(botID string, id uint) error {
	return m.db.Where("bot_id = ? AND id = ?", botID, id).Delete(&BotMemory{}).Error
}
