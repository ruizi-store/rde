package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerMultiplexer_ServeHTTP(t *testing.T) {
	mux := NewHandlerMultiplexer()

	// 添加 v1 处理器
	v1Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v1"))
	})
	mux.Handle("v1", v1Handler)

	// 添加 v2 处理器
	v2Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("v2"))
	})
	mux.Handle("v2", v2Handler)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"v1 path", "/v1/test", "v1"},
		{"v2 path", "/v2/test", "v2"},
		{"root path uses v1", "/", "v1"},
		{"unknown version uses v1", "/v3/test", "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Body.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, rec.Body.String())
			}
		})
	}
}

func TestHandlerMultiplexer_NotFound(t *testing.T) {
	mux := NewHandlerMultiplexer()
	// 不添加任何处理器

	req := httptest.NewRequest("GET", "/v1/test", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rec.Code)
	}
}

func TestHandlerMultiplexer_Handle(t *testing.T) {
	mux := NewHandlerMultiplexer()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	mux.Handle("test", handler)

	if mux.HandlerMap["test"] == nil {
		t.Error("Handler should be registered")
	}
}

func TestNewHandlerMultiplexer(t *testing.T) {
	mux := NewHandlerMultiplexer()

	if mux == nil {
		t.Error("Expected non-nil multiplexer")
	}

	if mux.HandlerMap == nil {
		t.Error("Expected non-nil HandlerMap")
	}
}
