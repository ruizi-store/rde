// Package terminal 终端模块 - 数据模型定义
package terminal

import (
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Session 终端会话
type Session struct {
	ID           string    `json:"id"`
	UserID       uint      `json:"userId"`
	Username     string    `json:"username"`
	Name         string    `json:"name"`
	Cols         uint16    `json:"cols"`
	Rows         uint16    `json:"rows"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActivity time.Time `json:"lastActivity"`

	// 内部字段（不序列化）
	pty  *os.File      `json:"-"`
	cmd  *exec.Cmd     `json:"-"`
	ws   *websocket.Conn `json:"-"`
	mu   sync.Mutex    `json:"-"`
	done chan struct{} `json:"-"`
}

// SessionInfo 会话信息（用于列表返回）
type SessionInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Cols         uint16    `json:"cols"`
	Rows         uint16    `json:"rows"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActivity time.Time `json:"lastActivity"`
}

// ToInfo 转换为会话信息
func (s *Session) ToInfo() *SessionInfo {
	return &SessionInfo{
		ID:           s.ID,
		Name:         s.Name,
		Cols:         s.Cols,
		Rows:         s.Rows,
		CreatedAt:    s.CreatedAt,
		LastActivity: s.LastActivity,
	}
}

// CreateSessionRequest 创建会话请求
type CreateSessionRequest struct {
	Cols  uint16 `json:"cols" binding:"omitempty,min=20,max=500"`
	Rows  uint16 `json:"rows" binding:"omitempty,min=5,max=200"`
	Shell string `json:"shell,omitempty"`
}

// ResizeRequest 调整大小请求
type ResizeRequest struct {
	Cols uint16 `json:"cols" binding:"required,min=20,max=500"`
	Rows uint16 `json:"rows" binding:"required,min=5,max=200"`
}

// ControlMessage WebSocket 控制消息
type ControlMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

// 配置常量
const (
	DefaultCols         = 80
	DefaultRows         = 24
	DefaultShell        = "/bin/bash"
	MaxSessionsPerUser  = 5
	IdleTimeoutMinutes  = 30
	WebSocketBufferSize = 64 * 1024 // 64KB
	HeartbeatInterval   = 30 * time.Second
	WriteWait           = 10 * time.Second
	PongWait            = 60 * time.Second
	PingPeriod          = (PongWait * 9) / 10
)
