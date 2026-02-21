// Package download WebSocket 连接管理
package download

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// 事件类型常量
const (
	EventTaskAdded     = "task:added"
	EventTaskProgress  = "task:progress"
	EventTaskCompleted = "task:completed"
	EventTaskPaused    = "task:paused"
	EventTaskResumed   = "task:resumed"
	EventTaskError     = "task:error"
	EventTaskRemoved   = "task:removed"
	EventStatsUpdate   = "stats:update"
	EventServiceStatus = "service:status"
)

// Event WebSocket 推送事件
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Time    int64       `json:"time"`
}

// Hub WebSocket 连接管理中心
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	logger     *zap.Logger
}

// Client WebSocket 客户端连接
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	logger *zap.Logger
}

// WebSocket 升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该限制
	},
}

// NewHub 创建 WebSocket Hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

// Run 运行 Hub，处理连接和消息
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Debug("client connected", zap.Int("total", len(h.clients)))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Debug("client disconnected", zap.Int("total", len(h.clients)))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 广播事件到所有连接的客户端
func (h *Hub) Broadcast(event Event) {
	if event.Time == 0 {
		event.Time = time.Now().UnixMilli()
	}
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("marshal event failed", zap.Error(err))
		return
	}
	h.broadcast <- data
}

// BroadcastTask 广播任务事件
func (h *Hub) BroadcastTask(eventType string, task *Task) {
	h.Broadcast(Event{
		Type:    eventType,
		Payload: task,
	})
}

// BroadcastStats 广播统计信息
func (h *Hub) BroadcastStats(stats *GlobalStat) {
	h.Broadcast(Event{
		Type:    EventStatsUpdate,
		Payload: stats,
	})
}

// BroadcastServiceStatus 广播服务状态
func (h *Hub) BroadcastServiceStatus(running bool) {
	h.Broadcast(Event{
		Type: EventServiceStatus,
		Payload: map[string]bool{
			"running": running,
		},
	})
}

// ClientCount 获取当前连接数
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWs 处理 WebSocket 连接请求
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		logger: h.logger,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// writePump 向客户端发送消息
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量发送队列中的消息
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// 发送心跳
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump 从客户端读取消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Debug("websocket read error", zap.Error(err))
			}
			break
		}
		// 目前客户端不需要发送消息，只接收推送
	}
}
