// Package ai 网关会话管理
package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GatewayHistoryMessage 网关消息历史记录
type GatewayHistoryMessage struct {
	ID        string       `json:"id"`
	Platform  PlatformType `json:"platform"`
	UserID    string       `json:"user_id"`
	UserName  string       `json:"user_name,omitempty"`
	Role      string       `json:"role"` // user/assistant/system
	Content   string       `json:"content"`
	Timestamp time.Time    `json:"timestamp"`
}

// ChatSession 聊天会话
type ChatSession struct {
	Platform      PlatformType            `json:"platform"`
	UserID        string                  `json:"user_id"`
	UserName      string                  `json:"user_name,omitempty"`
	Messages      []GatewayHistoryMessage `json:"messages"`
	FirstMessage  time.Time               `json:"first_message"`
	LastMessage   time.Time               `json:"last_message"`
	MessageCount  int                     `json:"message_count"`
	Preferences   *UserPreferences        `json:"preferences,omitempty"`
}

// UserPreferences 用户偏好
type UserPreferences struct {
	Language     string `json:"language,omitempty"`
	NotifyLevel  string `json:"notify_level,omitempty"`
	QuietMode    bool   `json:"quiet_mode"`
}

// SessionStore 会话存储
type SessionStore struct {
	logger     *zap.Logger
	dataDir    string
	sessions   map[string]*ChatSession // key: platform:user_id
	sessionsMu sync.RWMutex
	saveFile   string
}

// NewSessionStore 创建会话存储
func NewSessionStore(logger *zap.Logger, dataDir string) *SessionStore {
	store := &SessionStore{
		logger:   logger,
		dataDir:  dataDir,
		sessions: make(map[string]*ChatSession),
		saveFile: filepath.Join(dataDir, "gateway_sessions.json"),
	}
	store.load()
	return store
}

func (ss *SessionStore) load() {
	data, err := os.ReadFile(ss.saveFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &ss.sessions)
}

// Save 保存所有会话
func (ss *SessionStore) Save() {
	ss.sessionsMu.RLock()
	defer ss.sessionsMu.RUnlock()

	data, err := json.MarshalIndent(ss.sessions, "", "  ")
	if err != nil {
		ss.logger.Error("Failed to marshal sessions", zap.Error(err))
		return
	}
	if err := os.WriteFile(ss.saveFile, data, 0644); err != nil {
		ss.logger.Error("Failed to save sessions", zap.Error(err))
	}
}

// RecordMessage 记录消息
func (ss *SessionStore) RecordMessage(platform PlatformType, userID, userName, role, content string) {
	key := string(platform) + ":" + userID

	ss.sessionsMu.Lock()
	defer ss.sessionsMu.Unlock()

	session, exists := ss.sessions[key]
	if !exists {
		session = &ChatSession{
			Platform:     platform,
			UserID:       userID,
			UserName:     userName,
			Messages:     []GatewayHistoryMessage{},
			FirstMessage: time.Now(),
		}
		ss.sessions[key] = session
	}

	msg := GatewayHistoryMessage{
		ID:        time.Now().Format("20060102150405.000"),
		Platform:  platform,
		UserID:    userID,
		UserName:  userName,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	session.Messages = append(session.Messages, msg)
	session.LastMessage = time.Now()
	session.MessageCount++

	// 限制消息数量
	if len(session.Messages) > 500 {
		session.Messages = session.Messages[len(session.Messages)-200:]
	}

	if session.UserName == "" && userName != "" {
		session.UserName = userName
	}
}

// GetSession 获取会话
func (ss *SessionStore) GetSession(platform PlatformType, userID string) *ChatSession {
	key := string(platform) + ":" + userID
	ss.sessionsMu.RLock()
	defer ss.sessionsMu.RUnlock()
	return ss.sessions[key]
}

// GetAllSessions 获取所有会话
func (ss *SessionStore) GetAllSessions() []*ChatSession {
	ss.sessionsMu.RLock()
	defer ss.sessionsMu.RUnlock()

	result := make([]*ChatSession, 0, len(ss.sessions))
	for _, session := range ss.sessions {
		result = append(result, session)
	}
	return result
}

// GetStats 获取统计
func (ss *SessionStore) GetStats() map[string]interface{} {
	ss.sessionsMu.RLock()
	defer ss.sessionsMu.RUnlock()

	totalSessions := len(ss.sessions)
	totalMessages := 0
	platforms := make(map[string]int)

	for _, session := range ss.sessions {
		totalMessages += session.MessageCount
		platforms[string(session.Platform)]++
	}

	return map[string]interface{}{
		"total_sessions":  totalSessions,
		"total_messages":  totalMessages,
		"platforms":       platforms,
	}
}

// GetUserMessages 获取用户消息
func (ss *SessionStore) GetUserMessages(userID string) []GatewayHistoryMessage {
	ss.sessionsMu.RLock()
	defer ss.sessionsMu.RUnlock()

	var messages []GatewayHistoryMessage
	for _, session := range ss.sessions {
		if session.UserID == userID {
			messages = append(messages, session.Messages...)
		}
	}
	return messages
}

// ExportUserData GDPR 导出用户数据
func (ss *SessionStore) ExportUserData(userID string) map[string]interface{} {
	ss.sessionsMu.RLock()
	defer ss.sessionsMu.RUnlock()

	var sessions []*ChatSession
	for _, session := range ss.sessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return map[string]interface{}{
		"user_id":    userID,
		"sessions":   sessions,
		"exported_at": time.Now(),
	}
}

// DeleteUserData GDPR 删除用户数据
func (ss *SessionStore) DeleteUserData(userID string) int {
	ss.sessionsMu.Lock()
	defer ss.sessionsMu.Unlock()

	deleted := 0
	for key, session := range ss.sessions {
		if session.UserID == userID {
			delete(ss.sessions, key)
			deleted++
		}
	}
	return deleted
}

// ClearUserHistory 仅清除用户的消息历史，保留会话记录
func (ss *SessionStore) ClearUserHistory(platform PlatformType, userID string) {
	key := string(platform) + ":" + userID

	ss.sessionsMu.Lock()
	defer ss.sessionsMu.Unlock()

	if session, exists := ss.sessions[key]; exists {
		session.Messages = make([]GatewayHistoryMessage, 0)
		session.MessageCount = 0
	}
}

// GetRecentMessages 获取所有会话的最近消息
func (ss *SessionStore) GetRecentMessages(limit int) []GatewayHistoryMessage {
	ss.sessionsMu.RLock()
	defer ss.sessionsMu.RUnlock()

	// 收集所有消息
	var all []GatewayHistoryMessage
	for _, session := range ss.sessions {
		all = append(all, session.Messages...)
	}

	// 按时间排序（最新的在后）取最后 limit 条
	if len(all) > limit {
		// 简单取尾部，因为消息已经按时间顺序追加
		all = all[len(all)-limit:]
	}
	return all
}

// StartAutoSave 启动自动保存
func (ss *SessionStore) StartAutoSave(ctx <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx:
				ss.Save()
				return
			case <-ticker.C:
				ss.Save()
			}
		}
	}()
}
