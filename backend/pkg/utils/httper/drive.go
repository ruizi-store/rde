package httper

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// DriveClient 网盘/云存储客户端
type DriveClient struct {
	client    *Client
	baseURL   string
	authToken string
}

// DriveFile 文件信息
type DriveFile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	IsDir       bool      `json:"is_dir"`
	MimeType    string    `json:"mime_type"`
	ModifiedAt  time.Time `json:"modified_at"`
	CreatedAt   time.Time `json:"created_at"`
	DownloadURL string    `json:"download_url,omitempty"`
	ParentID    string    `json:"parent_id,omitempty"`
}

// DriveQuota 配额信息
type DriveQuota struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
}

// NewDriveClient 创建网盘客户端
func NewDriveClient(baseURL, authToken string) *DriveClient {
	return &DriveClient{
		client:    NewClient().WithTimeout(60 * time.Second),
		baseURL:   baseURL,
		authToken: authToken,
	}
}

// SetAuthToken 设置认证令牌
func (d *DriveClient) SetAuthToken(token string) *DriveClient {
	d.authToken = token
	return d
}

// request 发送请求
func (d *DriveClient) request(method, path string, body interface{}) (*Response, error) {
	url := d.baseURL + path

	d.client.WithHeader("Authorization", "Bearer "+d.authToken)
	d.client.WithHeader("Content-Type", "application/json")

	switch method {
	case "GET":
		return d.client.Get(url)
	case "POST":
		return d.client.Post(url, body)
	case "PUT":
		return d.client.Put(url, body)
	case "DELETE":
		return d.client.Delete(url)
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

// List 列出目录内容
func (d *DriveClient) List(path string) ([]DriveFile, error) {
	resp, err := d.request("GET", "/files?path="+path, nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to list files: status %d", resp.StatusCode)
	}

	var files []DriveFile
	if err := resp.JSON(&files); err != nil {
		return nil, err
	}

	return files, nil
}

// GetInfo 获取文件/目录信息
func (d *DriveClient) GetInfo(path string) (*DriveFile, error) {
	resp, err := d.request("GET", "/files/info?path="+path, nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to get file info: status %d", resp.StatusCode)
	}

	var file DriveFile
	if err := resp.JSON(&file); err != nil {
		return nil, err
	}

	return &file, nil
}

// Download 下载文件
func (d *DriveClient) Download(remotePath, localPath string) error {
	resp, err := d.request("GET", "/files/download?path="+remotePath, nil)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to download file: status %d", resp.StatusCode)
	}

	// 确保目录存在
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 创建本地文件并写入
	return os.WriteFile(localPath, resp.Body, 0644)
}

// Upload 上传文件
func (d *DriveClient) Upload(localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	url := d.baseURL + "/files/upload?path=" + remotePath
	d.client.WithHeader("Authorization", "Bearer "+d.authToken)
	resp, err := d.client.Post(url, map[string]string{
		"content": string(content),
	})
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to upload file: status %d", resp.StatusCode)
	}

	return nil
}

// Delete 删除文件/目录
func (d *DriveClient) Delete(path string) error {
	resp, err := d.request("DELETE", "/files?path="+path, nil)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to delete: status %d", resp.StatusCode)
	}

	return nil
}

// CreateDir 创建目录
func (d *DriveClient) CreateDir(path string) error {
	body := map[string]string{"path": path}
	resp, err := d.request("POST", "/files/mkdir", body)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to create directory: status %d", resp.StatusCode)
	}

	return nil
}

// Move 移动文件/目录
func (d *DriveClient) Move(srcPath, dstPath string) error {
	body := map[string]string{
		"src": srcPath,
		"dst": dstPath,
	}
	resp, err := d.request("POST", "/files/move", body)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to move: status %d", resp.StatusCode)
	}

	return nil
}

// Copy 复制文件/目录
func (d *DriveClient) Copy(srcPath, dstPath string) error {
	body := map[string]string{
		"src": srcPath,
		"dst": dstPath,
	}
	resp, err := d.request("POST", "/files/copy", body)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to copy: status %d", resp.StatusCode)
	}

	return nil
}

// Rename 重命名文件/目录
func (d *DriveClient) Rename(path, newName string) error {
	body := map[string]string{
		"path":     path,
		"new_name": newName,
	}
	resp, err := d.request("POST", "/files/rename", body)
	if err != nil {
		return err
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("failed to rename: status %d", resp.StatusCode)
	}

	return nil
}

// GetQuota 获取存储配额
func (d *DriveClient) GetQuota() (*DriveQuota, error) {
	resp, err := d.request("GET", "/quota", nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to get quota: status %d", resp.StatusCode)
	}

	var quota DriveQuota
	if err := resp.JSON(&quota); err != nil {
		return nil, err
	}

	return &quota, nil
}

// Search 搜索文件
func (d *DriveClient) Search(query string) ([]DriveFile, error) {
	resp, err := d.request("GET", "/files/search?q="+query, nil)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to search: status %d", resp.StatusCode)
	}

	var files []DriveFile
	if err := resp.JSON(&files); err != nil {
		return nil, err
	}

	return files, nil
}

// GetShareLink 获取分享链接
func (d *DriveClient) GetShareLink(path string, expiry time.Duration) (string, error) {
	body := map[string]interface{}{
		"path":   path,
		"expiry": int64(expiry.Seconds()),
	}
	resp, err := d.request("POST", "/share", body)
	if err != nil {
		return "", err
	}

	if !resp.IsSuccess() {
		return "", fmt.Errorf("failed to create share link: status %d", resp.StatusCode)
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return "", err
	}

	return result.URL, nil
}

