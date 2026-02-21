// Package ssh SSH远程连接模块 - 数据模型定义
package ssh

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Connection SSH连接配置（持久化存储）
type Connection struct {
	ID                  string `json:"id" gorm:"primaryKey;size:36"`
	Name                string `json:"name" gorm:"size:100;not null"`
	Host                string `json:"host" gorm:"size:255;not null"`
	Port                int    `json:"port" gorm:"default:22"`
	Username            string `json:"username" gorm:"size:100;not null"`
	AuthMethod          string `json:"auth_method" gorm:"size:20;not null"` // password | key
	EncryptedCredential string `json:"-" gorm:"type:text"`                  // AES加密的密码或私钥
	Passphrase          string `json:"-" gorm:"type:text"`                  // 私钥密码（加密）
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`
	LastUsedAt          int64  `json:"last_used_at"`
}

// TableName 指定表名
func (Connection) TableName() string {
	return "ssh_connections"
}

// Session SSH会话（内存中）
type Session struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	ConnName     string    `json:"conn_name"` // 连接名称，用于显示
	Host         string    `json:"host"`
	Username     string    `json:"username"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`

	// SSH 连接
	client     *ssh.Client   `json:"-"`
	sftpClient *sftp.Client  `json:"-"`
	ptySession *ssh.Session  `json:"-"`
	stdin      chan []byte   `json:"-"`
	done       chan struct{} `json:"-"`

	// WebSocket
	ws *websocket.Conn `json:"-"`
	mu sync.Mutex      `json:"-"`
}

// SessionInfo 会话信息（用于列表返回）
type SessionInfo struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connection_id"`
	ConnName     string    `json:"conn_name"`
	Host         string    `json:"host"`
	Username     string    `json:"username"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
}

// ToInfo 转换为会话信息
func (s *Session) ToInfo() *SessionInfo {
	return &SessionInfo{
		ID:           s.ID,
		ConnectionID: s.ConnectionID,
		ConnName:     s.ConnName,
		Host:         s.Host,
		Username:     s.Username,
		CreatedAt:    s.CreatedAt,
		LastActivity: s.LastActivity,
	}
}

// TransferTask 文件传输任务
type TransferTask struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	Type        string `json:"type"` // upload | download
	LocalPath   string `json:"local_path"`
	RemotePath  string `json:"remote_path"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	Transferred int64  `json:"transferred"`
	Status      string `json:"status"` // pending | running | done | failed | cancelled
	Error       string `json:"error,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	StartedAt   int64  `json:"started_at,omitempty"`
	FinishedAt  int64  `json:"finished_at,omitempty"`
}

// TransferQueue 传输队列
type TransferQueue struct {
	mu       sync.Mutex
	tasks    []*TransferTask
	notify   chan *TransferTask
	workers  int
	stopChan chan struct{}
}

// ==================== 请求/响应结构 ====================

// CreateConnectionRequest 创建连接请求
type CreateConnectionRequest struct {
	Name       string `json:"name" binding:"required,max=100"`
	Host       string `json:"host" binding:"required,max=255"`
	Port       int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username   string `json:"username" binding:"required,max=100"`
	AuthMethod string `json:"auth_method" binding:"required,oneof=password key"`
	Password   string `json:"password" binding:"omitempty"`   // 明文密码
	PrivateKey string `json:"private_key" binding:"omitempty"` // 私钥内容
	Passphrase string `json:"passphrase" binding:"omitempty"`  // 私钥密码
}

// UpdateConnectionRequest 更新连接请求
type UpdateConnectionRequest struct {
	Name       string `json:"name" binding:"omitempty,max=100"`
	Host       string `json:"host" binding:"omitempty,max=255"`
	Port       int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username   string `json:"username" binding:"omitempty,max=100"`
	AuthMethod string `json:"auth_method" binding:"omitempty,oneof=password key"`
	Password   string `json:"password" binding:"omitempty"`
	PrivateKey string `json:"private_key" binding:"omitempty"`
	Passphrase string `json:"passphrase" binding:"omitempty"`
}

// ConnectRequest 建立连接请求
type ConnectRequest struct {
	ConnectionID string `json:"connection_id" binding:"required"`
	Cols         uint16 `json:"cols" binding:"omitempty,min=20,max=500"`
	Rows         uint16 `json:"rows" binding:"omitempty,min=5,max=200"`
}

// TestConnectionRequest 测试连接请求
type TestConnectionRequest struct {
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username   string `json:"username" binding:"required"`
	AuthMethod string `json:"auth_method" binding:"required,oneof=password key"`
	Password   string `json:"password" binding:"omitempty"`
	PrivateKey string `json:"private_key" binding:"omitempty"`
	Passphrase string `json:"passphrase" binding:"omitempty"`
}

// ResizeRequest 调整终端大小
type ResizeRequest struct {
	Cols uint16 `json:"cols" binding:"required,min=20,max=500"`
	Rows uint16 `json:"rows" binding:"required,min=5,max=200"`
}

// FileInfo SFTP文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
	IsLink  bool   `json:"is_link"`
}

// ListDirRequest 列目录请求
type ListDirRequest struct {
	Path string `json:"path" binding:"required"`
}

// DownloadRequest 下载请求
type DownloadRequest struct {
	RemotePaths []string `json:"remote_paths" binding:"required,min=1"`
	LocalDir    string   `json:"local_dir" binding:"required"`
}

// UploadRequest 上传请求（用于非multipart场景）
type UploadRequest struct {
	LocalPaths []string `json:"local_paths" binding:"required,min=1"`
	RemoteDir  string   `json:"remote_dir" binding:"required"`
}

// MkdirRequest 创建目录请求
type MkdirRequest struct {
	Path string `json:"path" binding:"required"`
}

// RenameRequest 重命名请求
type RenameRequest struct {
	OldPath string `json:"old_path" binding:"required"`
	NewPath string `json:"new_path" binding:"required"`
}

// DeleteRequest 删除请求
type DeleteRequest struct {
	Paths []string `json:"paths" binding:"required,min=1"`
}

// ControlMessage WebSocket 控制消息
type ControlMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

// CreateTransferRequest 创建传输任务请求
type CreateTransferRequest struct {
	SessionID   string   `json:"session_id" binding:"required"`
	Type        string   `json:"type" binding:"required,oneof=upload download"`
	LocalPaths  []string `json:"local_paths" binding:"required_if=Type download,omitempty"`
	RemotePaths []string `json:"remote_paths" binding:"required_if=Type upload,omitempty"`
	LocalDir    string   `json:"local_dir" binding:"required_if=Type download,omitempty"`
	RemoteDir   string   `json:"remote_dir" binding:"required_if=Type upload,omitempty"`
}

// TransferProgress 传输进度（WebSocket推送）
type TransferProgress struct {
	TaskID      string  `json:"task_id"`
	FileName    string  `json:"file_name"`
	Type        string  `json:"type"`
	Size        int64   `json:"size"`
	Transferred int64   `json:"transferred"`
	Speed       float64 `json:"speed"` // bytes/s
	Status      string  `json:"status"`
	Error       string  `json:"error,omitempty"`
}

// 配置常量
const (
	DefaultPort         = 22
	DefaultCols         = 80
	DefaultRows         = 24
	MaxSessionsPerUser  = 10
	WebSocketBufferSize = 64 * 1024 // 64KB
	HeartbeatInterval   = 30 * time.Second
	WriteWait           = 10 * time.Second
	PongWait            = 60 * time.Second
	PingPeriod          = (PongWait * 9) / 10
	TransferWorkers     = 3 // 并发传输数
)
