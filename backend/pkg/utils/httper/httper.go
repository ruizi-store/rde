package httper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client HTTP 客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	headers    map[string]string
	timeout    time.Duration
	retryCount int
	retryDelay time.Duration
}

// NewClient 创建 HTTP 客户端
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers:    make(map[string]string),
		timeout:    30 * time.Second,
		retryCount: 0,
		retryDelay: time.Second,
	}
}

// WithTimeout 设置超时时间
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	c.timeout = timeout
	c.httpClient.Timeout = timeout
	return c
}

// WithBaseURL 设置基础 URL
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = strings.TrimSuffix(baseURL, "/")
	return c
}

// WithHeader 设置请求头
func (c *Client) WithHeader(key, value string) *Client {
	c.headers[key] = value
	return c
}

// WithHeaders 设置多个请求头
func (c *Client) WithHeaders(headers map[string]string) *Client {
	for k, v := range headers {
		c.headers[k] = v
	}
	return c
}

// WithRetry 设置重试配置
func (c *Client) WithRetry(count int, delay time.Duration) *Client {
	c.retryCount = count
	c.retryDelay = delay
	return c
}

// WithBearerToken 设置 Bearer Token
func (c *Client) WithBearerToken(token string) *Client {
	c.headers["Authorization"] = "Bearer " + token
	return c
}

// Response HTTP 响应
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// JSON 将响应体解析为 JSON
func (r *Response) JSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// String 返回响应体字符串
func (r *Response) String() string {
	return string(r.Body)
}

// IsSuccess 检查是否成功响应
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// buildURL 构建完整 URL
func (c *Client) buildURL(path string) string {
	if c.baseURL == "" {
		return path
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return c.baseURL + "/" + strings.TrimPrefix(path, "/")
}

// doRequest 执行请求
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*Response, error) {
	url := c.buildURL(path)

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryDelay)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, err
		}

		// 设置请求头
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		return &Response{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
		}, nil
	}

	return nil, lastErr
}

// Get 发送 GET 请求
func (c *Client) Get(path string) (*Response, error) {
	return c.GetContext(context.Background(), path)
}

// GetContext 发送带上下文的 GET 请求
func (c *Client) GetContext(ctx context.Context, path string) (*Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

// Post 发送 POST 请求
func (c *Client) Post(path string, body interface{}) (*Response, error) {
	return c.PostContext(context.Background(), path, body)
}

// PostContext 发送带上下文的 POST 请求
func (c *Client) PostContext(ctx context.Context, path string, body interface{}) (*Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
		c.headers["Content-Type"] = "application/json"
	}
	return c.doRequest(ctx, http.MethodPost, path, reader)
}

// PostForm 发送表单 POST 请求
func (c *Client) PostForm(path string, data url.Values) (*Response, error) {
	return c.PostFormContext(context.Background(), path, data)
}

// PostFormContext 发送带上下文的表单 POST 请求
func (c *Client) PostFormContext(ctx context.Context, path string, data url.Values) (*Response, error) {
	c.headers["Content-Type"] = "application/x-www-form-urlencoded"
	return c.doRequest(ctx, http.MethodPost, path, strings.NewReader(data.Encode()))
}

// Put 发送 PUT 请求
func (c *Client) Put(path string, body interface{}) (*Response, error) {
	return c.PutContext(context.Background(), path, body)
}

// PutContext 发送带上下文的 PUT 请求
func (c *Client) PutContext(ctx context.Context, path string, body interface{}) (*Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
		c.headers["Content-Type"] = "application/json"
	}
	return c.doRequest(ctx, http.MethodPut, path, reader)
}

// Delete 发送 DELETE 请求
func (c *Client) Delete(path string) (*Response, error) {
	return c.DeleteContext(context.Background(), path)
}

// DeleteContext 发送带上下文的 DELETE 请求
func (c *Client) DeleteContext(ctx context.Context, path string) (*Response, error) {
	return c.doRequest(ctx, http.MethodDelete, path, nil)
}

// Patch 发送 PATCH 请求
func (c *Client) Patch(path string, body interface{}) (*Response, error) {
	return c.PatchContext(context.Background(), path, body)
}

// PatchContext 发送带上下文的 PATCH 请求
func (c *Client) PatchContext(ctx context.Context, path string, body interface{}) (*Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
		c.headers["Content-Type"] = "application/json"
	}
	return c.doRequest(ctx, http.MethodPatch, path, reader)
}

// Download 下载文件
func (c *Client) Download(url, savePath string, onProgress func(current, total int64)) error {
	return c.DownloadContext(context.Background(), url, savePath, onProgress)
}

// DownloadContext 带上下文下载文件
func (c *Client) DownloadContext(ctx context.Context, downloadURL, savePath string, onProgress func(current, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// 确保目录存在
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 如果有进度回调
	if onProgress != nil {
		total := resp.ContentLength
		var current int64
		buf := make([]byte, 32*1024)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				_, writeErr := file.Write(buf[:n])
				if writeErr != nil {
					return writeErr
				}
				current += int64(n)
				onProgress(current, total)
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
		}
		return nil
	}

	_, err = io.Copy(file, resp.Body)
	return err
}

// 便捷函数

// Get 快捷 GET 请求
func Get(url string) (*Response, error) {
	return NewClient().Get(url)
}

// Post 快捷 POST 请求
func Post(url string, body interface{}) (*Response, error) {
	return NewClient().Post(url, body)
}

// GetJSON 获取并解析 JSON
func GetJSON(url string, v interface{}) error {
	resp, err := Get(url)
	if err != nil {
		return err
	}
	return resp.JSON(v)
}

// PostJSON 发送 JSON 并解析响应
func PostJSON(url string, body interface{}, v interface{}) error {
	resp, err := Post(url, body)
	if err != nil {
		return err
	}
	return resp.JSON(v)
}

// ==================== 存储挂载相关（兼容 CasaOS）====================

// MountList 挂载列表
type MountList struct {
	MountPoints []MountPoint `json:"mountPoints"`
}

// MountPoint 挂载点信息
type MountPoint struct {
	MountPoint string `json:"MountPoint"`
	Fs         string `json:"Fs"`
}

// RemotesResult 远程配置结果
type RemotesResult struct {
	Remotes map[string]map[string]string `json:"remotes"`
}

// Mount 挂载存储（占位符实现）
func Mount(mountPoint, fs string) error {
	// TODO: 实现 rclone mount 调用
	return nil
}

// Unmount 卸载存储（占位符实现）
func Unmount(mountPoint string) error {
	// TODO: 实现 rclone unmount 调用
	return nil
}

// GetMountList 获取挂载列表（占位符实现）
func GetMountList() (MountList, error) {
	// TODO: 实现 rclone mount/listmounts 调用
	return MountList{}, nil
}

// CreateConfig 创建配置（占位符实现）
func CreateConfig(data map[string]string, name string, t string) error {
	// TODO: 实现 rclone config create 调用
	return nil
}

// GetRemotes 获取远程配置（占位符实现）
func GetRemotes() (RemotesResult, error) {
	// TODO: 实现 rclone config dump 调用
	return RemotesResult{}, nil
}

// OasisGet 获取远程数据（兼容函数）
func OasisGet(url string) string {
	resp, err := Get(url)
	if err != nil {
		return ""
	}
	return resp.String()
}

// GetConfigByName 根据名称获取配置（CasaOS 兼容）
func GetConfigByName(name string) (map[string]string, error) {
	// TODO: 实现 rclone config get 调用
	return map[string]string{}, nil
}

// GetAllConfigName 获取所有配置名称（CasaOS 兼容）
func GetAllConfigName() (RemotesResult, error) {
	// TODO: 实现 rclone listremotes 调用
	return RemotesResult{Remotes: map[string]map[string]string{}}, nil
}

// DeleteConfigByName 删除配置（CasaOS 兼容）
func DeleteConfigByName(name string) error {
	// TODO: 实现 rclone config delete 调用
	return nil
}

