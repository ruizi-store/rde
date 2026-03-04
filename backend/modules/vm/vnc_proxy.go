// Package vm VNC WebSocket 代理
package vm

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// VNCProxy VNC WebSocket 代理
type VNCProxy struct {
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

// NewVNCProxy 创建 VNC 代理
func NewVNCProxy(logger *zap.Logger) *VNCProxy {
	return &VNCProxy{
		logger: logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			Subprotocols: []string{"binary"},
		},
	}
}

// HandleWebSocket 处理 WebSocket 连接，将流量代理到 VNC 服务器
func (p *VNCProxy) HandleWebSocket(c *gin.Context, vncHost string, vncPort int) {
	ws, err := p.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		p.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer ws.Close()

	vncAddr := fmt.Sprintf("%s:%d", vncHost, vncPort)
	vnc, err := net.DialTimeout("tcp", vncAddr, 10*time.Second)
	if err != nil {
		p.logger.Error("Failed to connect to VNC server",
			zap.String("addr", vncAddr),
			zap.Error(err),
		)
		ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Cannot connect to VNC"))
		return
	}
	defer vnc.Close()

	p.logger.Info("VNC proxy connected",
		zap.String("vnc_addr", vncAddr),
		zap.String("client", c.ClientIP()),
	)

	var wg sync.WaitGroup
	wg.Add(2)

	// WebSocket -> VNC
	go func() {
		defer wg.Done()
		p.wsToVNC(ws, vnc)
	}()

	// VNC -> WebSocket
	go func() {
		defer wg.Done()
		p.vncToWS(vnc, ws)
	}()

	wg.Wait()
	p.logger.Info("VNC proxy disconnected", zap.String("client", c.ClientIP()))
}

// wsToVNC 从 WebSocket 读取数据发送到 VNC
func (p *VNCProxy) wsToVNC(ws *websocket.Conn, vnc net.Conn) {
	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				p.logger.Debug("WebSocket read error", zap.Error(err))
			}
			return
		}

		if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
			if _, err = vnc.Write(data); err != nil {
				p.logger.Debug("VNC write error", zap.Error(err))
				return
			}
		}
	}
}

// vncToWS 从 VNC 读取数据发送到 WebSocket
func (p *VNCProxy) vncToWS(vnc net.Conn, ws *websocket.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := vnc.Read(buf)
		if err != nil {
			if err != io.EOF {
				p.logger.Debug("VNC read error", zap.Error(err))
			}
			return
		}

		if err = ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
			p.logger.Debug("WebSocket write error", zap.Error(err))
			return
		}
	}
}

// VNCTokenManager VNC 令牌管理器
type VNCTokenManager struct {
	tokens map[string]*VNCToken
	mu     sync.RWMutex
}

// NewVNCTokenManager 创建令牌管理器
func NewVNCTokenManager() *VNCTokenManager {
	mgr := &VNCTokenManager{
		tokens: make(map[string]*VNCToken),
	}
	go mgr.cleanupExpired()
	return mgr
}

// GenerateToken 生成令牌
func (m *VNCTokenManager) GenerateToken(vmID string, vncPort int, duration time.Duration) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	m.tokens[token] = &VNCToken{
		Token:     token,
		VMID:      vmID,
		VNCPort:   vncPort,
		ExpiresAt: time.Now().Add(duration),
	}

	return token
}

// ValidateToken 验证令牌
func (m *VNCTokenManager) ValidateToken(token string) (*VNCToken, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vncToken, exists := m.tokens[token]
	if !exists {
		return nil, false
	}

	if time.Now().After(vncToken.ExpiresAt) {
		return nil, false
	}

	return vncToken, true
}

// cleanupExpired 清理过期令牌
func (m *VNCTokenManager) cleanupExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for token, vncToken := range m.tokens {
			if now.After(vncToken.ExpiresAt) {
				delete(m.tokens, token)
			}
		}
		m.mu.Unlock()
	}
}
