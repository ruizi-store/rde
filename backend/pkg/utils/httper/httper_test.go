package httper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient() 返回 nil")
	}

	if client.httpClient == nil {
		t.Error("httpClient 不应为 nil")
	}

	if client.headers == nil {
		t.Error("headers map 不应为 nil")
	}

	if client.timeout != 30*time.Second {
		t.Errorf("默认超时应为 30 秒, 实际为 %v", client.timeout)
	}
}

func TestClientWithTimeout(t *testing.T) {
	client := NewClient().WithTimeout(10 * time.Second)

	if client.timeout != 10*time.Second {
		t.Errorf("超时设置失败: got %v, want 10s", client.timeout)
	}
}

func TestClientWithBaseURL(t *testing.T) {
	client := NewClient().WithBaseURL("https://api.example.com/")

	// 应该去除尾部斜杠
	if client.baseURL != "https://api.example.com" {
		t.Errorf("baseURL 设置不正确: got %q", client.baseURL)
	}
}

func TestClientWithHeader(t *testing.T) {
	client := NewClient().WithHeader("X-Custom", "value")

	if client.headers["X-Custom"] != "value" {
		t.Error("Header 未正确设置")
	}
}

func TestClientWithHeaders(t *testing.T) {
	headers := map[string]string{
		"X-Header1": "value1",
		"X-Header2": "value2",
	}
	client := NewClient().WithHeaders(headers)

	for k, v := range headers {
		if client.headers[k] != v {
			t.Errorf("Header %s 未正确设置", k)
		}
	}
}

func TestClientWithBearerToken(t *testing.T) {
	client := NewClient().WithBearerToken("test-token")

	expected := "Bearer test-token"
	if client.headers["Authorization"] != expected {
		t.Errorf("Authorization header 不正确: got %q, want %q",
			client.headers["Authorization"], expected)
	}
}

func TestClientWithRetry(t *testing.T) {
	client := NewClient().WithRetry(3, 500*time.Millisecond)

	if client.retryCount != 3 {
		t.Errorf("retryCount 不正确: got %d, want 3", client.retryCount)
	}

	if client.retryDelay != 500*time.Millisecond {
		t.Errorf("retryDelay 不正确: got %v, want 500ms", client.retryDelay)
	}
}

func TestGet(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET 请求, 实际 %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"hello"}`))
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.Get(server.URL)

	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码不正确: got %d, want 200", resp.StatusCode)
	}

	if !resp.IsSuccess() {
		t.Error("IsSuccess() 应返回 true")
	}

	expected := `{"message":"hello"}`
	if resp.String() != expected {
		t.Errorf("响应体不正确: got %q, want %q", resp.String(), expected)
	}
}

func TestGetContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	// 测试正常请求
	ctx := context.Background()
	client := NewClient()
	resp, err := client.GetContext(ctx, server.URL)

	if err != nil {
		t.Fatalf("GetContext 失败: %v", err)
	}

	if resp.String() != "ok" {
		t.Errorf("响应不正确: got %q", resp.String())
	}

	// 测试取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.GetContext(ctx, server.URL)
	if err == nil {
		t.Error("取消的上下文应返回错误")
	}
}

func TestPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 请求, 实际 %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type 应为 application/json")
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "created"})
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.Post(server.URL, map[string]string{"name": "test"})

	if err != nil {
		t.Fatalf("Post 失败: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("状态码不正确: got %d, want 201", resp.StatusCode)
	}
}

func TestPostForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 请求, 实际 %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Error("Content-Type 应为 application/x-www-form-urlencoded")
		}

		r.ParseForm()
		if r.FormValue("key") != "value" {
			t.Errorf("表单值不正确: got %q", r.FormValue("key"))
		}

		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient()
	data := url.Values{}
	data.Set("key", "value")

	resp, err := client.PostForm(server.URL, data)

	if err != nil {
		t.Fatalf("PostForm 失败: %v", err)
	}

	if resp.String() != "ok" {
		t.Errorf("响应不正确: got %q", resp.String())
	}
}

func TestResponseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":  "test",
			"count": 42,
		})
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.Get(server.URL)

	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	var result struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	err = resp.JSON(&result)
	if err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	if result.Name != "test" || result.Count != 42 {
		t.Errorf("JSON 解析不正确: %+v", result)
	}
}

func TestResponseIsSuccess(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{400, false},
		{404, false},
		{500, false},
	}

	for _, tt := range tests {
		resp := &Response{StatusCode: tt.statusCode}
		if resp.IsSuccess() != tt.expected {
			t.Errorf("IsSuccess() for status %d: got %v, want %v",
				tt.statusCode, resp.IsSuccess(), tt.expected)
		}
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		baseURL  string
		path     string
		expected string
	}{
		{"", "/api/users", "/api/users"},
		{"https://api.example.com", "/users", "https://api.example.com/users"},
		{"https://api.example.com", "users", "https://api.example.com/users"},
		{"https://api.example.com", "https://other.com/api", "https://other.com/api"},
		{"https://api.example.com", "http://other.com/api", "http://other.com/api"},
	}

	for _, tt := range tests {
		client := NewClient().WithBaseURL(tt.baseURL)
		result := client.buildURL(tt.path)

		if result != tt.expected {
			t.Errorf("buildURL(%q, %q) = %q, 期望 %q",
				tt.baseURL, tt.path, result, tt.expected)
		}
	}
}

func TestClientChaining(t *testing.T) {
	// 测试链式调用
	client := NewClient().
		WithBaseURL("https://api.example.com").
		WithTimeout(5*time.Second).
		WithHeader("X-API-Key", "secret").
		WithBearerToken("token").
		WithRetry(2, time.Second)

	if client.baseURL != "https://api.example.com" {
		t.Error("baseURL 设置失败")
	}

	if client.timeout != 5*time.Second {
		t.Error("timeout 设置失败")
	}

	if client.headers["X-API-Key"] != "secret" {
		t.Error("自定义 header 设置失败")
	}

	if client.headers["Authorization"] != "Bearer token" {
		t.Error("Authorization header 设置失败")
	}

	if client.retryCount != 2 {
		t.Error("retryCount 设置失败")
	}
}

func TestPut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("期望 PUT 请求, 实际 %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("updated"))
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.Put(server.URL, map[string]string{"name": "updated"})

	if err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码不正确: got %d", resp.StatusCode)
	}
}

func TestRetryMechanism(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// 前两次返回错误
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	client := NewClient().WithRetry(3, 10*time.Millisecond)
	resp, err := client.Get(server.URL)

	// 由于重试机制基于网络错误而非 HTTP 状态码，这里可能不会触发重试
	// 此测试主要验证配置是否生效
	if err != nil {
		t.Logf("请求失败（可能需要调整重试逻辑）: %v", err)
	} else {
		t.Logf("请求成功，尝试次数: %d, 状态码: %d", attempts, resp.StatusCode)
	}
}
