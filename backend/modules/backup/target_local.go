package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// LocalTarget 本地存储目标
type LocalTarget struct {
	config LocalTargetConfig
}

// Configure 配置本地目标
func (t *LocalTarget) Configure(config string) error {
	if err := json.Unmarshal([]byte(config), &t.config); err != nil {
		return fmt.Errorf("解析本地配置失败: %w", err)
	}
	if t.config.Path == "" {
		return fmt.Errorf("本地路径不能为空")
	}
	return nil
}

// Test 测试本地目标
func (t *LocalTarget) Test() *TargetTestResponse {
	// 检查路径是否存在
	stat, err := os.Stat(t.config.Path)
	if os.IsNotExist(err) {
		// 尝试创建
		if err := os.MkdirAll(t.config.Path, 0755); err != nil {
			return &TargetTestResponse{
				Success: false,
				Message: "目录不存在且无法创建: " + err.Error(),
			}
		}
		stat, _ = os.Stat(t.config.Path)
	} else if err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "检查路径失败: " + err.Error(),
		}
	}

	if !stat.IsDir() {
		return &TargetTestResponse{
			Success: false,
			Message: "指定路径不是目录",
		}
	}

	// 检查可写性
	testFile := filepath.Join(t.config.Path, ".rde-backup-test")
	f, err := os.Create(testFile)
	if err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "目录不可写: " + err.Error(),
		}
	}
	f.Close()
	os.Remove(testFile)

	// 获取可用空间
	var freeSpace int64
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(t.config.Path, &statfs); err == nil {
		freeSpace = int64(statfs.Bavail) * int64(statfs.Bsize)
	}

	return &TargetTestResponse{
		Success:   true,
		Message:   "连接成功",
		FreeSpace: freeSpace,
	}
}

// Upload 上传文件到本地目标
func (t *LocalTarget) Upload(localPath, remoteName string, progress func(int)) (string, error) {
	destPath := filepath.Join(t.config.Path, remoteName)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", err
	}

	src, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return "", err
	}

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// 带进度复制
	reader := &ProgressReader{
		Reader:   src,
		Total:    stat.Size(),
		Callback: progress,
	}

	if _, err := io.Copy(dst, reader); err != nil {
		os.Remove(destPath)
		return "", err
	}

	return destPath, nil
}

// Download 从本地目标下载文件
func (t *LocalTarget) Download(remotePath, localPath string, progress func(int)) error {
	src, err := os.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	reader := &ProgressReader{
		Reader:   src,
		Total:    stat.Size(),
		Callback: progress,
	}

	_, err = io.Copy(dst, reader)
	return err
}

// Delete 删除本地文件
func (t *LocalTarget) Delete(remotePath string) error {
	return os.Remove(remotePath)
}

// List 列出备份文件
func (t *LocalTarget) List() ([]RemoteFile, error) {
	var files []RemoteFile

	entries, err := os.ReadDir(t.config.Path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, RemoteFile{
			Name:    entry.Name(),
			Path:    filepath.Join(t.config.Path, entry.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})
	}

	return files, nil
}
