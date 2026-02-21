// Package download Aria2 事件监听
package download

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Aria2Notification Aria2 通知消息
type Aria2Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []struct {
		GID string `json:"gid"`
	} `json:"params"`
}

// EventListener Aria2 事件监听器
type EventListener struct {
	service    *Service
	hub        *Hub
	storage    *Storage
	logger     *zap.Logger
	conn       *websocket.Conn
	done       chan struct{}
	mu         sync.Mutex
	reconnect  bool
	rpcPort    int
	lastStats  *GlobalStat
	statsTicker *time.Ticker
}

// NewEventListener 创建事件监听器
func NewEventListener(service *Service, hub *Hub, storage *Storage, logger *zap.Logger, rpcPort int) *EventListener {
	return &EventListener{
		service:   service,
		hub:       hub,
		storage:   storage,
		logger:    logger,
		done:      make(chan struct{}),
		reconnect: true,
		rpcPort:   rpcPort,
	}
}

// Start 启动事件监听
func (l *EventListener) Start() error {
	go l.connect()
	go l.statsLoop()
	return nil
}

// Stop 停止事件监听
func (l *EventListener) Stop() {
	l.reconnect = false
	close(l.done)

	if l.statsTicker != nil {
		l.statsTicker.Stop()
	}

	l.mu.Lock()
	if l.conn != nil {
		l.conn.Close()
	}
	l.mu.Unlock()
}

// connect 连接到 Aria2 WebSocket
func (l *EventListener) connect() {
	for l.reconnect {
		select {
		case <-l.done:
			return
		default:
		}

		url := fmt.Sprintf("ws://127.0.0.1:%d/jsonrpc", l.rpcPort)
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			l.logger.Debug("aria2 websocket connect failed, retrying...", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}

		l.mu.Lock()
		l.conn = conn
		l.mu.Unlock()

		l.logger.Info("aria2 event listener connected")
		l.listen()
	}
}

// listen 监听消息
func (l *EventListener) listen() {
	defer func() {
		l.mu.Lock()
		if l.conn != nil {
			l.conn.Close()
			l.conn = nil
		}
		l.mu.Unlock()
	}()

	for {
		select {
		case <-l.done:
			return
		default:
		}

		l.mu.Lock()
		conn := l.conn
		l.mu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				l.logger.Debug("aria2 websocket read error", zap.Error(err))
			}
			return
		}

		l.handleMessage(message)
	}
}

// handleMessage 处理 Aria2 通知
func (l *EventListener) handleMessage(data []byte) {
	var notification Aria2Notification
	if err := json.Unmarshal(data, &notification); err != nil {
		l.logger.Debug("parse aria2 notification failed", zap.Error(err))
		return
	}

	if len(notification.Params) == 0 {
		return
	}

	gid := notification.Params[0].GID
	l.logger.Debug("aria2 event received",
		zap.String("method", notification.Method),
		zap.String("gid", gid))

	// 获取任务详情
	task, err := l.service.GetTask(gid)
	if err != nil {
		l.logger.Debug("get task failed", zap.String("gid", gid), zap.Error(err))
		return
	}

	var eventType string
	switch notification.Method {
	case "aria2.onDownloadStart":
		eventType = EventTaskAdded

	case "aria2.onDownloadPause":
		eventType = EventTaskPaused

	case "aria2.onDownloadStop":
		eventType = EventTaskRemoved

	case "aria2.onDownloadComplete", "aria2.onBtDownloadComplete":
		eventType = EventTaskCompleted
		// 保存到历史记录
		if err := l.storage.SaveHistory(task); err != nil {
			l.logger.Error("save history failed", zap.Error(err))
		}

	case "aria2.onDownloadError":
		eventType = EventTaskError
		// 更新历史记录状态
		if err := l.storage.UpdateHistoryStatus(gid, "error", task.ErrorMessage); err != nil {
			l.logger.Error("update history status failed", zap.Error(err))
		}
	}

	if eventType != "" {
		l.hub.BroadcastTask(eventType, task)
	}
}

// statsLoop 定期广播统计信息
func (l *EventListener) statsLoop() {
	l.statsTicker = time.NewTicker(1 * time.Second)
	defer l.statsTicker.Stop()

	for {
		select {
		case <-l.done:
			return
		case <-l.statsTicker.C:
			l.broadcastStats()
		}
	}
}

// broadcastStats 广播统计信息
func (l *EventListener) broadcastStats() {
	if l.hub.ClientCount() == 0 {
		return // 没有客户端连接，跳过
	}

	stats, err := l.service.GetStats()
	if err != nil {
		return
	}

	// 检查是否有变化
	if l.lastStats != nil &&
		l.lastStats.DownloadSpeed == stats.DownloadSpeed &&
		l.lastStats.UploadSpeed == stats.UploadSpeed &&
		l.lastStats.NumActive == stats.NumActive &&
		l.lastStats.NumWaiting == stats.NumWaiting {
		return // 没有变化，跳过
	}

	l.lastStats = stats
	l.hub.BroadcastStats(stats)
}

// ProgressBroadcaster 进度广播器
type ProgressBroadcaster struct {
	service *Service
	hub     *Hub
	logger  *zap.Logger
	done    chan struct{}
	ticker  *time.Ticker
}

// NewProgressBroadcaster 创建进度广播器
func NewProgressBroadcaster(service *Service, hub *Hub, logger *zap.Logger) *ProgressBroadcaster {
	return &ProgressBroadcaster{
		service: service,
		hub:     hub,
		logger:  logger,
		done:    make(chan struct{}),
	}
}

// Start 启动进度广播
func (p *ProgressBroadcaster) Start() {
	p.ticker = time.NewTicker(500 * time.Millisecond)
	go p.loop()
}

// Stop 停止进度广播
func (p *ProgressBroadcaster) Stop() {
	close(p.done)
	if p.ticker != nil {
		p.ticker.Stop()
	}
}

// loop 广播循环
func (p *ProgressBroadcaster) loop() {
	for {
		select {
		case <-p.done:
			return
		case <-p.ticker.C:
			p.broadcast()
		}
	}
}

// broadcast 广播所有活跃任务的进度
func (p *ProgressBroadcaster) broadcast() {
	if p.hub.ClientCount() == 0 {
		return
	}

	tasks, err := p.service.GetActiveTasks()
	if err != nil {
		return
	}

	for _, task := range tasks {
		p.hub.BroadcastTask(EventTaskProgress, task)
	}
}
