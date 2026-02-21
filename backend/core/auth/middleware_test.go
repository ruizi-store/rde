package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(tm *TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func TestMiddleware_ValidToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/protected", Middleware(tm), func(c *gin.Context) {
		userID := GetUserID(c)
		username := GetUsername(c)
		role := GetRole(c)

		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"username": username,
			"role":     role,
		})
	})

	token, _ := tm.GenerateAccessToken("1", "testuser", "admin")

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_MissingToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/protected", Middleware(tm), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_InvalidToken(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/protected", Middleware(tm), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_TokenFromQuery(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/protected", Middleware(tm), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": GetUserID(c)})
	})

	token, _ := tm.GenerateAccessToken("1", "testuser", "admin")

	req := httptest.NewRequest("GET", "/protected?token="+token, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_Skipper(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	middlewareConfig := &MiddlewareConfig{
		Skipper: SkipPaths("/public"),
	}

	router.GET("/public/hello", MiddlewareWithConfig(tm, middlewareConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/protected", MiddlewareWithConfig(tm, middlewareConfig), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 公开路径不需要令牌
	req := httptest.NewRequest("GET", "/public/hello", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 受保护路径需要令牌
	req = httptest.NewRequest("GET", "/protected", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOptionalMiddleware(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/optional", OptionalMiddleware(tm), func(c *gin.Context) {
		userID := GetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	// 无令牌 - 应该通过，但 user_id 为 0
	req := httptest.NewRequest("GET", "/optional", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 有令牌 - 应该通过，user_id 有值
	token, _ := tm.GenerateAccessToken("42", "testuser", "admin")
	req = httptest.NewRequest("GET", "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/admin", Middleware(tm), RequireRole("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 管理员可以访问
	adminToken, _ := tm.GenerateAccessToken("1", "admin", "admin")
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 普通用户不能访问
	userToken, _ := tm.GenerateAccessToken("2", "user", "user")
	req = httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireAdmin(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/admin-only", Middleware(tm), RequireAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 管理员可以访问
	adminToken, _ := tm.GenerateAccessToken("1", "admin", "admin")
	req := httptest.NewRequest("GET", "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetClaims(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/claims", Middleware(tm), func(c *gin.Context) {
		claims := GetClaims(c)
		require.NotNil(t, claims)
		c.JSON(http.StatusOK, gin.H{
			"user_id":  claims.UserID,
			"username": claims.Username,
			"role":     claims.Role,
			"issuer":   claims.Issuer,
		})
	})

	token, _ := tm.GenerateAccessToken("1", "testuser", "admin")
	req := httptest.NewRequest("GET", "/claims", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsAuthenticated(t *testing.T) {
	config := &Config{
		JWTSecret:     "test-secret",
		AccessExpiry:  time.Hour,
		RefreshExpiry: 24 * time.Hour,
		Issuer:        "test",
	}
	tm := NewTokenManager(config)
	router := setupTestRouter(tm)

	router.GET("/check", OptionalMiddleware(tm), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": IsAuthenticated(c),
			"is_admin":      IsAdmin(c),
		})
	})

	// 未认证
	req := httptest.NewRequest("GET", "/check", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 已认证（管理员）
	token, _ := tm.GenerateAccessToken("1", "admin", "admin")
	req = httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setup    func(req *http.Request)
		expected string
	}{
		{
			name: "from Authorization header with Bearer",
			setup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer mytoken")
			},
			expected: "mytoken",
		},
		{
			name: "from Authorization header without Bearer",
			setup: func(req *http.Request) {
				req.Header.Set("Authorization", "mytoken")
			},
			expected: "mytoken",
		},
		{
			name: "from query parameter",
			setup: func(req *http.Request) {
				q := req.URL.Query()
				q.Add("token", "querytoken")
				req.URL.RawQuery = q.Encode()
			},
			expected: "querytoken",
		},
		{
			name:     "no token",
			setup:    func(req *http.Request) {},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/test", func(c *gin.Context) {
				token := ExtractToken(c)
				c.String(http.StatusOK, token)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			tt.setup(req)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}
