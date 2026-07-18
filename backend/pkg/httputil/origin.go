package httputil

import (
	"net/http"
	"net/url"
	"strings"
)

// SameOrigin 校验 Origin 与请求 Host 是否同源；无 Origin 时放行（非浏览器客户端）
func SameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
