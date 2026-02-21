// Package notification WebSocket Hub
package notification

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// WebSocketHub 管理所有 WebSocket 连接
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mutex      sync.RWMutex
}

// WebSocketClient 单个 WebSocket 客户端
type WebSocketClient struct {
	hub    *WebSocketHub
	conn   *websocket.Conn
	send   chan []byte
	userID string
}

// WebSocketMessage WebSocket 消息格式
type WebSocketMessage struct {
	Type string      `json:"type"` // notification, unread_count
	Data interface{} `json:"data"`
}

// NewWebSocketHub 创建新的 WebSocket Hub
func NewWebSocketHub() *WebSocketHub {
	hub := &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
	go hub.run()
	return hub
}

// run 运行 Hub
func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// HandleWebSocket 处理 WebSocket 连接
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &WebSocketClient{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// BroadcastNotification 广播新通知
func (h *WebSocketHub) BroadcastNotification(notification *Notification) {
	msg := WebSocketMessage{
		Type: "notification",
		Data: notification,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.broadcast <- data
}

// BroadcastUnreadCount 广播未读数量
func (h *WebSocketHub) BroadcastUnreadCount(userID string, count int64) {
	msg := WebSocketMessage{
		Type: "unread_count",
		Data: map[string]interface{}{
			"count": count,
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mutex.RLock()
	for client := range h.clients {
		if client.userID == userID || userID == "" {
			select {
			case client.send <- data:
			default:
			}
		}
	}
	h.mutex.RUnlock()
}

// SendToUser 发送消息给指定用户
func (h *WebSocketHub) SendToUser(userID string, message *WebSocketMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	h.mutex.RLock()
	for client := range h.clients {
		if client.userID == userID {
			select {
			case client.send <- data:
			default:
			}
		}
	}
	h.mutex.RUnlock()
}

// Broadcast 广播任意消息给所有连接的客户端
func (h *WebSocketHub) Broadcast(message *WebSocketMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}
	h.broadcast <- data
}

// readPump 从 WebSocket 读取消息
func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// 暂不处理客户端消息
	}
}

// writePump 向 WebSocket 写入消息
func (c *WebSocketClient) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}
