package httputil

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HandlerMultiplexer HTTP 处理器多路复用器 - CasaOS 兼容
type HandlerMultiplexer struct {
	HandlerMap    map[string]http.Handler
	StaticPath    string // 静态文件目录
	StaticHandler http.Handler
}

// ServeHTTP 实现 http.Handler 接口
func (m *HandlerMultiplexer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// API 路由 - 以 /v1, /v2 等开头
	if strings.HasPrefix(path, "/v1") || strings.HasPrefix(path, "/v2") {
		// 移除开头的斜杠
		trimmed := strings.TrimPrefix(path, "/")
		parts := strings.SplitN(trimmed, "/", 2)
		version := parts[0]

		if handler, ok := m.HandlerMap[version]; ok {
			handler.ServeHTTP(w, r)
			return
		}
	}

	// 静态文件服务
	if m.StaticHandler != nil && m.StaticPath != "" {
		// 检查请求的文件是否存在
		filePath := filepath.Join(m.StaticPath, path)
		if _, err := os.Stat(filePath); err == nil {
			m.StaticHandler.ServeHTTP(w, r)
			return
		}

		// SPA 回退：对于不存在的路径返回 index.html
		if !strings.HasPrefix(path, "/api") && !strings.Contains(path, ".") {
			r.URL.Path = "/"
			m.StaticHandler.ServeHTTP(w, r)
			return
		}

		// 尝试提供静态文件
		m.StaticHandler.ServeHTTP(w, r)
		return
	}

	// 默认 API 处理
	if handler, ok := m.HandlerMap["v1"]; ok {
		handler.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

// Handle 添加处理器
func (m *HandlerMultiplexer) Handle(pattern string, handler http.Handler) {
	if m.HandlerMap == nil {
		m.HandlerMap = make(map[string]http.Handler)
	}
	m.HandlerMap[pattern] = handler
}

// NewHandlerMultiplexer 创建新的多路复用器
func NewHandlerMultiplexer() *HandlerMultiplexer {
	return &HandlerMultiplexer{
		HandlerMap: make(map[string]http.Handler),
	}
}
