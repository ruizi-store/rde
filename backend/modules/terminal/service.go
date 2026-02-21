// Package terminal 终端模块 - 业务逻辑层
package terminal

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Service 终端服务
type Service struct {
	sessionManager *SessionManager
	logger         *zap.Logger
	mu             sync.Mutex
}

// NewService 创建终端服务
func NewService(logger *zap.Logger) *Service {
	return &Service{
		sessionManager: NewSessionManager(logger),
		logger:         logger,
	}
}

// CreateSession 创建终端会话
func (s *Service) CreateSession(userID uint, username string, req *CreateSessionRequest) (*Session, error) {
	return s.sessionManager.CreateSession(userID, username, req)
}

// GetSessions 获取会话列表
func (s *Service) GetSessions(userID uint) []*SessionInfo {
	// 管理员可以看到所有会话，这里简化处理，返回该用户的会话
	return s.sessionManager.GetUserSessions(userID)
}

// GetAllSessions 获取所有会话（管理员用）
func (s *Service) GetAllSessions() []*SessionInfo {
	return s.sessionManager.GetAllSessions()
}

// GetSession 获取单个会话
func (s *Service) GetSession(sessionID string) (*Session, bool) {
	return s.sessionManager.GetSession(sessionID)
}

// Resize 调整终端大小
func (s *Service) Resize(sessionID string, cols, rows uint16) error {
	return s.sessionManager.Resize(sessionID, cols, rows)
}

// CloseSession 关闭会话
func (s *Service) CloseSession(sessionID string) error {
	return s.sessionManager.CloseSession(sessionID)
}

// CloseAllSessions 关闭所有会话（禁用终端时调用）
func (s *Service) CloseAllSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions := s.sessionManager.GetAllSessions()
	for _, session := range sessions {
		if err := s.sessionManager.CloseSession(session.ID); err != nil {
			s.logger.Warn("Failed to close session", zap.String("sessionId", session.ID), zap.Error(err))
		}
	}
	s.logger.Info("All terminal sessions closed", zap.Int("count", len(sessions)))
}

// AttachWebSocket 将 WebSocket 连接到会话
func (s *Service) AttachWebSocket(sessionID string, ws *websocket.Conn) error {
	session, ok := s.sessionManager.GetSession(sessionID)
	if !ok {
		return ErrSessionNotFound
	}

	session.mu.Lock()
	if session.pty == nil {
		session.mu.Unlock()
		return ErrSessionClosed
	}
	session.ws = ws
	session.mu.Unlock()

	// 设置 WebSocket 参数
	ws.SetReadLimit(WebSocketBufferSize)
	ws.SetReadDeadline(time.Now().Add(PongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(PongWait))
		s.sessionManager.UpdateActivity(sessionID)
		return nil
	})

	// 启动双向桥接
	done := make(chan struct{})

	// PTY → WebSocket
	go s.pipeToWebSocket(session, ws, done)

	// WebSocket → PTY (在当前 goroutine 运行)
	s.pipeFromWebSocket(session, ws, done)

	return nil
}

// pipeToWebSocket 将 PTY 输出转发到 WebSocket
func (s *Service) pipeToWebSocket(session *Session, ws *websocket.Conn, done chan struct{}) {
	pingTicker := time.NewTicker(PingPeriod)
	defer pingTicker.Stop()

	buf := make([]byte, 4096)
	dataCh := make(chan []byte, 10)
	errCh := make(chan error, 1)

	// 启动 PTY 读取协程
	go func() {
		for {
			session.mu.Lock()
			ptyFile := session.pty
			session.mu.Unlock()

			if ptyFile == nil {
				errCh <- io.EOF
				return
			}

			n, err := ptyFile.Read(buf)
			if err != nil {
				errCh <- err
				return
			}

			if n > 0 {
				// 复制数据避免竞争
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case dataCh <- data:
				case <-session.done:
					return
				case <-done:
					return
				}
			}
		}
	}()

	for {
		select {
		case <-session.done:
			return
		case <-done:
			return
		case <-pingTicker.C:
			// 发送心跳
			ws.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.logger.Debug("WebSocket ping failed", zap.Error(err))
				return
			}
		case data := <-dataCh:
			ws.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := ws.WriteMessage(websocket.BinaryMessage, data); err != nil {
				s.logger.Debug("WebSocket write error", zap.Error(err))
				return
			}
			s.sessionManager.UpdateActivity(session.ID)
		case err := <-errCh:
			if err != io.EOF {
				s.logger.Debug("PTY read error", zap.Error(err))
			}
			return
		}
	}
}

// pipeFromWebSocket 将 WebSocket 输入转发到 PTY
func (s *Service) pipeFromWebSocket(session *Session, ws *websocket.Conn, done chan struct{}) {
	defer close(done)

	for {
		select {
		case <-session.done:
			return
		default:
		}

		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Debug("WebSocket read error", zap.Error(err))
			}
			return
		}

		session.mu.Lock()
		ptyFile := session.pty
		session.mu.Unlock()

		if ptyFile == nil {
			return
		}

		switch msgType {
		case websocket.BinaryMessage:
			// 用户输入 (二进制)
			if _, err := ptyFile.Write(msg); err != nil {
				s.logger.Debug("PTY write error", zap.Error(err))
				return
			}
			s.sessionManager.UpdateActivity(session.ID)

		case websocket.TextMessage:
			// 先尝试解析为控制消息
			var ctrlMsg ControlMessage
			if err := json.Unmarshal(msg, &ctrlMsg); err == nil && ctrlMsg.Type != "" {
				// 是控制消息
				switch ctrlMsg.Type {
				case "resize":
					if err := s.sessionManager.Resize(session.ID, ctrlMsg.Cols, ctrlMsg.Rows); err != nil {
						s.logger.Debug("Resize failed", zap.Error(err))
					}
				}
			} else {
				// 不是控制消息，当作用户输入处理
				if _, err := ptyFile.Write(msg); err != nil {
					s.logger.Debug("PTY write error", zap.Error(err))
					return
				}
				s.sessionManager.UpdateActivity(session.ID)
			}
		}
	}
}

// Stop 停止服务
func (s *Service) Stop() {
	s.sessionManager.Stop()
}

// 错误定义
var (
	ErrSessionNotFound = &ServiceError{Code: "SESSION_NOT_FOUND", Message: "Terminal session not found"}
	ErrSessionClosed   = &ServiceError{Code: "SESSION_CLOSED", Message: "Terminal session is closed"}
	ErrAccessDenied    = &ServiceError{Code: "ACCESS_DENIED", Message: "Access denied"}
)

// ServiceError 服务错误
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}
