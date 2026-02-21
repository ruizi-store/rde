package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/studio-b12/gowebdav"
)

// WebDAVTarget WebDAV 存储目标
type WebDAVTarget struct {
	config WebDAVTargetConfig
	client *gowebdav.Client
}

// Configure 配置 WebDAV 目标
func (t *WebDAVTarget) Configure(config string) error {
	if err := json.Unmarshal([]byte(config), &t.config); err != nil {
		return fmt.Errorf("解析 WebDAV 配置失败: %w", err)
	}

	if t.config.URL == "" {
		return fmt.Errorf("WebDAV URL 不能为空")
	}

	// 创建客户端
	t.client = gowebdav.NewClient(t.config.URL, t.config.Username, t.config.Password)

	return nil
}

// Test 测试 WebDAV 连接
func (t *WebDAVTarget) Test() *TargetTestResponse {
	if t.client == nil {
		return &TargetTestResponse{
			Success: false,
			Message: "客户端未初始化",
		}
	}

	// 测试连接
	err := t.client.Connect()
	if err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "连接失败: " + err.Error(),
		}
	}

	// 检查/创建备份目录
	backupPath := t.getBackupPath()
	err = t.client.MkdirAll(backupPath, 0755)
	if err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "创建备份目录失败: " + err.Error(),
		}
	}

	// 尝试获取目录信息
	_, err = t.client.Stat(backupPath)
	if err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "访问备份目录失败: " + err.Error(),
		}
	}

	return &TargetTestResponse{
		Success: true,
		Message: "连接成功",
	}
}

// Upload 上传文件到 WebDAV
func (t *WebDAVTarget) Upload(localPath, remoteName string, progress func(int)) (string, error) {
	if t.client == nil {
		return "", fmt.Errorf("客户端未初始化")
	}

	// 打开本地文件
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 确保远程目录存在
	backupPath := t.getBackupPath()
	if err := t.client.MkdirAll(backupPath, 0755); err != nil {
		return "", fmt.Errorf("创建远程目录失败: %w", err)
	}

	remotePath := path.Join(backupPath, remoteName)

	// 带进度上传
	reader := &ProgressReader{
		Reader:   file,
		Total:    stat.Size(),
		Callback: progress,
	}

	err = t.client.WriteStream(remotePath, reader, 0644)
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	return remotePath, nil
}

// Download 从 WebDAV 下载文件
func (t *WebDAVTarget) Download(remotePath, localPath string, progress func(int)) error {
	if t.client == nil {
		return fmt.Errorf("客户端未初始化")
	}

	// 获取远程文件信息
	info, err := t.client.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("获取远程文件信息失败: %w", err)
	}

	// 打开远程文件流
	reader, err := t.client.ReadStream(remotePath)
	if err != nil {
		return fmt.Errorf("读取远程文件失败: %w", err)
	}
	defer reader.Close()

	// 确保本地目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	// 创建本地文件
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer file.Close()

	// 带进度下载
	progressReader := &ProgressReader{
		Reader:   reader,
		Total:    info.Size(),
		Callback: progress,
	}

	_, err = io.Copy(file, progressReader)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	return nil
}

// Delete 删除 WebDAV 文件
func (t *WebDAVTarget) Delete(remotePath string) error {
	if t.client == nil {
		return fmt.Errorf("客户端未初始化")
	}

	return t.client.Remove(remotePath)
}

// List 列出备份文件
func (t *WebDAVTarget) List() ([]RemoteFile, error) {
	if t.client == nil {
		return nil, fmt.Errorf("客户端未初始化")
	}

	backupPath := t.getBackupPath()
	files, err := t.client.ReadDir(backupPath)
	if err != nil {
		return nil, err
	}

	var result []RemoteFile
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		result = append(result, RemoteFile{
			Name:    f.Name(),
			Path:    path.Join(backupPath, f.Name()),
			Size:    f.Size(),
			ModTime: f.ModTime().Unix(),
		})
	}

	return result, nil
}

// getBackupPath 获取备份路径
func (t *WebDAVTarget) getBackupPath() string {
	p := t.config.Path
	if p == "" {
		p = "/rde-backups"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
