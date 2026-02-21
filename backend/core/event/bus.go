// Package event 提供模块间异步通信的事件总线
package event

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Event 事件结构
type Event struct {
	Type      string      `json:"type"`      // 事件类型，如 "user.created"
	Source    string      `json:"source"`    // 来源模块 ID
	Data      interface{} `json:"data"`      // 事件数据
	Timestamp int64       `json:"timestamp"` // Unix 毫秒时间戳
}

// Handler 事件处理函数
type Handler func(event Event)

// Bus 事件总线
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]handlerEntry
	logger   *zap.Logger
	async    bool // 是否异步处理事件
}

type handlerEntry struct {
	handler Handler
	id      string // 订阅者标识，用于取消订阅
}

// NewBus 创建新的事件总线
func NewBus(logger *zap.Logger) *Bus {
	return &Bus{
		handlers: make(map[string][]handlerEntry),
		logger:   logger,
		async:    true,
	}
}

// NewSyncBus 创建同步事件总线（主要用于测试）
func NewSyncBus(logger *zap.Logger) *Bus {
	b := NewBus(logger)
	b.async = false
	return b
}

// Publish 发布事件
func (b *Bus) Publish(eventType string, data interface{}) {
	b.PublishFrom("", eventType, data)
}

// PublishFrom 从指定模块发布事件
func (b *Bus) PublishFrom(source, eventType string, data interface{}) {
	event := Event{
		Type:      eventType,
		Source:    source,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	b.mu.RLock()
	handlers := make([]handlerEntry, len(b.handlers[eventType]))
	copy(handlers, b.handlers[eventType])
	
	// 同时获取通配符订阅者
	wildcardHandlers := make([]handlerEntry, len(b.handlers["*"]))
	copy(wildcardHandlers, b.handlers["*"])
	b.mu.RUnlock()

	if b.logger != nil {
		b.logger.Debug("Event published",
			zap.String("type", eventType),
			zap.String("source", source),
			zap.Int("handlers", len(handlers)+len(wildcardHandlers)),
		)
	}

	allHandlers := append(handlers, wildcardHandlers...)
	
	for _, entry := range allHandlers {
		if b.async {
			go b.safeCall(entry.handler, event)
		} else {
			b.safeCall(entry.handler, event)
		}
	}
}

// Subscribe 订阅事件
// 返回取消订阅的函数
func (b *Bus) Subscribe(eventType string, handler Handler) func() {
	return b.SubscribeWithID("", eventType, handler)
}

// SubscribeWithID 使用指定 ID 订阅事件
// subscriberID 用于标识订阅者，便于调试和取消订阅
func (b *Bus) SubscribeWithID(subscriberID, eventType string, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry := handlerEntry{
		handler: handler,
		id:      subscriberID,
	}

	b.handlers[eventType] = append(b.handlers[eventType], entry)

	if b.logger != nil {
		b.logger.Debug("Event subscribed",
			zap.String("type", eventType),
			zap.String("subscriber", subscriberID),
		)
	}

	// 记录当前索引用于取消订阅
	index := len(b.handlers[eventType]) - 1

	// 返回取消订阅函数
	return func() {
		b.unsubscribe(eventType, index)
	}
}

// SubscribeAll 订阅所有事件
func (b *Bus) SubscribeAll(handler Handler) func() {
	return b.Subscribe("*", handler)
}

// unsubscribe 取消订阅（使用索引标记）
func (b *Bus) unsubscribe(eventType string, handlerIndex int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers := b.handlers[eventType]
	if handlerIndex >= 0 && handlerIndex < len(handlers) {
		b.handlers[eventType] = append(handlers[:handlerIndex], handlers[handlerIndex+1:]...)
	}
}

// safeCall 安全调用处理函数，捕获 panic
func (b *Bus) safeCall(handler Handler, event Event) {
	defer func() {
		if r := recover(); r != nil {
			if b.logger != nil {
				b.logger.Error("Event handler panicked",
					zap.String("type", event.Type),
					zap.Any("panic", r),
				)
			}
		}
	}()

	handler(event)
}

// HandlerCount 返回指定事件类型的处理器数量（用于测试）
func (b *Bus) HandlerCount(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType])
}

// Clear 清除所有订阅（用于测试）
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make(map[string][]handlerEntry)
}
