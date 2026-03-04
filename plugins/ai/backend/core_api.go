// Package ai RDE Core API 客户端
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// CoreAPI RDE Core API 客户端
type CoreAPI struct {
	baseURL      string
	packageToken string
	packageID    string
	client       *http.Client
	logger       *zap.Logger
}

// CoreAPIResponse Core API 响应
type CoreAPIResponse struct {
	Success int                    `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

// NewCoreAPI 创建 Core API 客户端
func NewCoreAPI(logger *zap.Logger) *CoreAPI {
	baseURL := os.Getenv("RDE_API_BASE")
	if baseURL == "" {
		baseURL = "http://localhost:3080"
	}

	return &CoreAPI{
		baseURL:      baseURL,
		packageToken: os.Getenv("RDE_PACKAGE_TOKEN"),
		packageID:    os.Getenv("RDE_PACKAGE_ID"),
		client:       &http.Client{Timeout: 60 * time.Second},
		logger:       logger,
	}
}

// Call 调用 Core API
func (c *CoreAPI) Call(method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求失败: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.packageToken != "" {
		req.Header.Set("X-Package-Token", c.packageToken)
	}
	if c.packageID != "" {
		req.Header.Set("X-Package-ID", c.packageID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(respBody))
	}

	var result CoreAPIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Success != 200 {
		return nil, fmt.Errorf("API 返回错误 (%d): %s", result.Success, result.Message)
	}

	return result.Data, nil
}

// ControlService 控制服务
func (c *CoreAPI) ControlService(service, action string) error {
	_, err := c.Call("POST", "/api/v1/privilege/service/control", map[string]string{
		"service": service,
		"action":  action,
	})
	return err
}

// GetServiceStatus 获取服务状态
func (c *CoreAPI) GetServiceStatus(service string) (map[string]interface{}, error) {
	return c.Call("GET", fmt.Sprintf("/api/v1/privilege/service/%s/status", service), nil)
}

// StartOllamaService 启动 Ollama 服务
func (c *CoreAPI) StartOllamaService() error {
	c.logger.Info("Starting Ollama service via Core API...")
	return c.ControlService("rde-pkg-ai-ollama", "start")
}

// StopOllamaService 停止 Ollama 服务
func (c *CoreAPI) StopOllamaService() error {
	return c.ControlService("rde-pkg-ai-ollama", "stop")
}

// IsOllamaRunning 检查 Ollama 服务是否运行
func (c *CoreAPI) IsOllamaRunning() bool {
	status, err := c.GetServiceStatus("rde-pkg-ai-ollama")
	if err != nil {
		c.logger.Debug("Failed to get Ollama service status", zap.Error(err))
		return false
	}

	if running, ok := status["running"].(bool); ok {
		return running
	}
	return false
}
