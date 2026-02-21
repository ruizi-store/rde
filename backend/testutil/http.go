package testutil

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// SetupTestRouter 创建测试模式的 gin 路由器
func SetupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// DoRequest 执行 HTTP 请求并返回响应记录器
func DoRequest(router *gin.Engine, method, path string, body ...http.Handler) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	router.ServeHTTP(w, req)
	return w
}

// DoRequestWithBody 执行带 body 的 HTTP 请求
func DoRequestWithBody(t *testing.T, router *gin.Engine, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(w, req)
	return w
}

// RequireStatus 断言 HTTP 响应状态码
func RequireStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	require.Equal(t, expected, w.Code, "response body: %s", w.Body.String())
}
