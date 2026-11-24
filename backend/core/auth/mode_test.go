package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestResolveAuthMode(t *testing.T) {
	t.Setenv("RDE_AUTH_MODE", "")
	assert.Equal(t, AuthModeNone, ResolveAuthMode(), "未设置默认免登录")

	t.Setenv("RDE_AUTH_MODE", "local")
	assert.Equal(t, AuthModeLocal, ResolveAuthMode())

	t.Setenv("RDE_AUTH_MODE", "LOCAL")
	assert.Equal(t, AuthModeLocal, ResolveAuthMode(), "大小写不敏感")

	t.Setenv("RDE_AUTH_MODE", "none")
	assert.Equal(t, AuthModeNone, ResolveAuthMode())

	t.Setenv("RDE_AUTH_MODE", "ldap") // 非法值回落默认
	assert.Equal(t, AuthModeNone, ResolveAuthMode())

	t.Setenv("RDE_AUTH_MODE", " none ")
	assert.Equal(t, AuthModeNone, ResolveAuthMode(), "容忍空白")
}

func TestNoneMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NoneMiddleware())
	got := map[string]string{}
	router.GET("/x", func(c *gin.Context) {
		for _, key := range []string{ContextKeyUserID, ContextKeyUsername, ContextKeyRole, "user_id", "username", "role"} {
			if v, ok := c.Get(key); ok {
				if s, isStr := v.(string); isStr {
					got[key] = s
				}
			}
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, NoneUserID, got[ContextKeyUserID])
	assert.Equal(t, NoneUserID, got["user_id"])
	assert.Equal(t, NoneUsername, got[ContextKeyUsername])
	assert.Equal(t, NoneUsername, got["username"])
	assert.Equal(t, "admin", got[ContextKeyRole])
	assert.Equal(t, "admin", got["role"])
}
