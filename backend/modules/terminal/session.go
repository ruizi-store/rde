// Package terminal 终端模块 - 会话管理
package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SessionManager 会话管理器
type SessionManager struct {
	sessions  map[string]*Session // sessionID -> Session
	userIndex map[uint][]string   // userID -> sessionIDs
	mu        sync.RWMutex
	logger    *zap.Logger

	// 配置
	maxSessionsPerUser int
	idleTimeout        time.Duration
	defaultShell       string

	// 清理协程控制
	cleanupDone chan struct{}
}

// NewSessionManager 创建会话管理器
func NewSessionManager(logger *zap.Logger) *SessionManager {
	sm := &SessionManager{
		sessions:           make(map[string]*Session),
		userIndex:          make(map[uint][]string),
		logger:             logger,
		maxSessionsPerUser: MaxSessionsPerUser,
		idleTimeout:        time.Duration(IdleTimeoutMinutes) * time.Minute,
		defaultShell:       DefaultShell,
		cleanupDone:        make(chan struct{}),
	}

	// 启动清理协程
	go sm.cleanupLoop()

	return sm
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(userID uint, username string, req *CreateSessionRequest) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查用户会话数量
	if len(sm.userIndex[userID]) >= sm.maxSessionsPerUser {
		return nil, fmt.Errorf("exceeded maximum sessions per user (%d)", sm.maxSessionsPerUser)
	}

	// 确定参数
	cols := req.Cols
	if cols == 0 {
		cols = DefaultCols
	}
	rows := req.Rows
	if rows == 0 {
		rows = DefaultRows
	}
	shell := req.Shell
	if shell == "" {
		shell = sm.defaultShell
	}

	// 尝试使用用户在系统中配置的 shell
	if req.Shell == "" {
		if u, err := user.Lookup(username); err == nil && u.HomeDir != "" {
			// 从 /etc/passwd 读取用户的默认 shell
			if userShell := getUserShell(u.Username); userShell != "" {
				shell = userShell
			}
		}
	}

	// 验证 shell 是否存在且可用
	if !isValidShell(shell) {
		shell = "/bin/sh" // 回退到基础 shell
	}

	// 创建 PTY（以登录用户身份运行）
	ptmx, cmd, err := sm.createPTY(shell, cols, rows, username)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}

	// 生成会话 ID
	sessionID := "term_" + uuid.New().String()[:8]
	sessionNum := len(sm.userIndex[userID]) + 1

	session := &Session{
		ID:           sessionID,
		UserID:       userID,
		Username:     username,
		Name:         fmt.Sprintf("Terminal %d", sessionNum),
		Cols:         cols,
		Rows:         rows,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		pty:          ptmx,
		cmd:          cmd,
		done:         make(chan struct{}),
	}

	// 注册会话
	sm.sessions[sessionID] = session
	sm.userIndex[userID] = append(sm.userIndex[userID], sessionID)

	sm.logger.Info("Terminal session created",
		zap.String("sessionId", sessionID),
		zap.Uint("userId", userID),
		zap.String("username", username),
		zap.String("shell", shell),
	)

	// 监控进程退出
	go sm.watchProcess(session)

	return session, nil
}

// createPTY 创建伪终端（以指定用户身份运行）
func (sm *SessionManager) createPTY(shell string, cols, rows uint16, username string) (*os.File, *exec.Cmd, error) {
	cmd := exec.Command(shell)

	// 查找系统用户，以登录用户身份运行 shell
	var homeDir string
	if u, err := user.Lookup(username); err == nil {
		uid, _ := strconv.ParseUint(u.Uid, 10, 32)
		gid, _ := strconv.ParseUint(u.Gid, 10, 32)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
		}
		cmd.Dir = u.HomeDir
		homeDir = u.HomeDir
		sm.logger.Info("Terminal will run as user",
			zap.String("username", username),
			zap.String("uid", u.Uid),
			zap.String("home", u.HomeDir),
		)
	} else {
		sm.logger.Warn("System user not found, running as service user",
			zap.String("username", username),
			zap.Error(err),
		)
		homeDir = os.Getenv("HOME")
	}

	cmd.Env = []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"LANG=en_US.UTF-8",
		"HOME=" + homeDir,
		"USER=" + username,
		"LOGNAME=" + username,
		"SHELL=" + shell,
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	})
	if err != nil {
		return nil, nil, err
	}

	return ptmx, cmd, nil
}

// getUserShell 从 /etc/passwd 获取用户的默认 shell
func getUserShell(username string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	prefix := username + ":"
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			fields := strings.Split(line, ":")
			if len(fields) >= 7 && fields[6] != "" {
				if _, err := os.Stat(fields[6]); err == nil {
					return fields[6]
				}
			}
		}
	}
	return ""
}

// isValidShell 检查 shell 是否存在且可用
// 排除 nologin, false 等伪 shell
func isValidShell(shell string) bool {
	// 检查文件是否存在
	if _, err := os.Stat(shell); os.IsNotExist(err) {
		return false
	}

	// 排除不可用的伪 shell
	invalidShells := []string{
		"nologin",
		"false",
		"true",
		"sync",
		"halt",
		"shutdown",
	}

	shellBase := filepath.Base(shell)
	for _, invalid := range invalidShells {
		if shellBase == invalid {
			return false
		}
	}

	return true
}

// watchProcess 监控进程退出
func (sm *SessionManager) watchProcess(session *Session) {
	if session.cmd == nil || session.cmd.Process == nil {
		return
	}

	// 等待进程退出
	_ = session.cmd.Wait()

	sm.logger.Info("Terminal process exited",
		zap.String("sessionId", session.ID),
	)

	// 关闭会话
	sm.CloseSession(session.ID)
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[sessionID]
	return session, ok
}

// GetUserSessions 获取用户的所有会话
func (sm *SessionManager) GetUserSessions(userID uint) []*SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessionIDs := sm.userIndex[userID]
	result := make([]*SessionInfo, 0, len(sessionIDs))

	for _, id := range sessionIDs {
		if session, ok := sm.sessions[id]; ok {
			result = append(result, session.ToInfo())
		}
	}

	return result
}

// GetAllSessions 获取所有会话（管理员用）
func (sm *SessionManager) GetAllSessions() []*SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*SessionInfo, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		result = append(result, session.ToInfo())
	}

	return result
}

// Resize 调整终端大小
func (sm *SessionManager) Resize(sessionID string, cols, rows uint16) error {
	sm.mu.RLock()
	session, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.pty == nil {
		return fmt.Errorf("session PTY is closed")
	}

	err := pty.Setsize(session.pty, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	})
	if err != nil {
		return fmt.Errorf("failed to resize PTY: %w", err)
	}

	session.Cols = cols
	session.Rows = rows
	session.LastActivity = time.Now()

	sm.logger.Debug("Terminal resized",
		zap.String("sessionId", sessionID),
		zap.Uint16("cols", cols),
		zap.Uint16("rows", rows),
	)

	return nil
}

// UpdateActivity 更新会话活动时间
func (sm *SessionManager) UpdateActivity(sessionID string) {
	sm.mu.RLock()
	session, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if ok {
		session.mu.Lock()
		session.LastActivity = time.Now()
		session.mu.Unlock()
	}
}

// CloseSession 关闭会话
func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	session, ok := sm.sessions[sessionID]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 从索引中移除
	delete(sm.sessions, sessionID)
	userIDs := sm.userIndex[session.UserID]
	for i, id := range userIDs {
		if id == sessionID {
			sm.userIndex[session.UserID] = append(userIDs[:i], userIDs[i+1:]...)
			break
		}
	}
	sm.mu.Unlock()

	// 关闭会话资源
	session.mu.Lock()
	defer session.mu.Unlock()

	// 通知关闭
	select {
	case <-session.done:
		// 已关闭
	default:
		close(session.done)
	}

	// 关闭 WebSocket
	if session.ws != nil {
		session.ws.Close()
		session.ws = nil
	}

	// 关闭 PTY
	if session.pty != nil {
		session.pty.Close()
		session.pty = nil
	}

	// 终止进程
	if session.cmd != nil && session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}

	sm.logger.Info("Terminal session closed",
		zap.String("sessionId", sessionID),
	)

	return nil
}

// cleanupLoop 清理过期会话
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-sm.cleanupDone:
			return
		case <-ticker.C:
			sm.cleanupIdleSessions()
		}
	}
}

// cleanupIdleSessions 清理空闲会话
func (sm *SessionManager) cleanupIdleSessions() {
	sm.mu.RLock()
	var toClose []string
	now := time.Now()

	for id, session := range sm.sessions {
		session.mu.Lock()
		idle := now.Sub(session.LastActivity) > sm.idleTimeout
		session.mu.Unlock()

		if idle {
			toClose = append(toClose, id)
		}
	}
	sm.mu.RUnlock()

	// 关闭空闲会话
	for _, id := range toClose {
		sm.logger.Info("Closing idle terminal session",
			zap.String("sessionId", id),
		)
		sm.CloseSession(id)
	}
}

// Stop 停止会话管理器
func (sm *SessionManager) Stop() {
	// 停止清理协程
	close(sm.cleanupDone)

	// 关闭所有会话
	sm.mu.RLock()
	sessionIDs := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	sm.mu.RUnlock()

	for _, id := range sessionIDs {
		sm.CloseSession(id)
	}

	sm.logger.Info("Terminal session manager stopped")
}
