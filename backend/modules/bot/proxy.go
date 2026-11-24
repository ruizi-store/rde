// Package bot 沙箱串流代理
//
// 将沙箱容器内 KasmVNC 的 web 客户端（含 WebSocket）经控制面反向代理，
// 统一走 /api/v1/bots/:id/vnc 路径，由全局 JWT 中间件鉴权（iframe 用 ?token=）。
package bot

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StreamProxy KasmVNC 反向代理
type StreamProxy struct {
	service *Service
	logger  *zap.Logger
}

// NewStreamProxy 创建代理
func NewStreamProxy(service *Service, logger *zap.Logger) *StreamProxy {
	return &StreamProxy{service: service, logger: logger}
}

// Handle 处理串流请求
func (p *StreamProxy) Handle(c *gin.Context, botID string) {
	addr, err := p.service.GetSandboxAddr(c.Request.Context(), botID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	target, err := url.Parse(addr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无效的沙箱地址"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = 100 * time.Millisecond // WebSocket/长连接需要即时刷新
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.logger.Warn("bot stream proxy error",
			zap.String("bot", botID), zap.String("path", r.URL.Path), zap.Error(err))
		w.WriteHeader(http.StatusBadGateway)
	}

	// 剥离 token 查询参数，不透传给沙箱
	q := cloneQuery(c.Request.URL.Query())
	q.Del("token")
	c.Request.URL.RawQuery = q.Encode()

	// 剥离路由前缀：/api/v1/bots/<id>/vnc/<rest> -> /<rest>
	// KasmVNC 自带 web 客户端在 / 提供，WebSocket 在 /websockify
	sub := strings.TrimPrefix(c.Param("path"), "/")
	c.Request.URL.Path = "/" + sub

	p.logger.Debug("bot stream proxied",
		zap.String("bot", botID), zap.String("target", target.String()), zap.String("path", c.Request.URL.Path))

	proxy.ServeHTTP(c.Writer, c.Request)
}

// cloneQuery 复制查询参数（避免直接修改原请求）
func cloneQuery(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}
