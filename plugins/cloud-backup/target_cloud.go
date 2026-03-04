package cloud_backup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// CloudTarget 云备份目标实现（通过 RDE Cloud API）
type CloudTarget struct {
	cfg    CloudTargetConfig
	client *http.Client
}

// Configure 配置云备份目标
func (t *CloudTarget) Configure(configStr string) error {
	if err := json.Unmarshal([]byte(configStr), &t.cfg); err != nil {
		return fmt.Errorf("解析云备份配置失败: %w", err)
	}
	if t.cfg.CloudToken == "" {
		return fmt.Errorf("云端 Token 不能为空，请先绑定会员")
	}
	if t.cfg.CloudURL == "" {
		t.cfg.CloudURL = "https://rde.lidj.cn"
	}
	t.client = &http.Client{Timeout: 5 * time.Minute}
	return nil
}

// Test 测试连接
func (t *CloudTarget) Test() *TargetTestResponse {
	resp, err := t.apiGet("/api/v1/membership/status")
	if err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: fmt.Sprintf("云端连接失败: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return &TargetTestResponse{
			Success: false,
			Message: "会员认证失败，请重新绑定",
		}
	}
	if resp.StatusCode == http.StatusForbidden {
		return &TargetTestResponse{
			Success: false,
			Message: "当前套餐不支持云备份",
		}
	}

	return &TargetTestResponse{
		Success: true,
		Message: "云备份连接正常",
	}
}

// Upload 上传备份文件到云端
func (t *CloudTarget) Upload(localPath, remoteName string, progress func(int)) (string, error) {
	// 1. 计算 SHA256 校验和
	checksum, err := CalculateSHA256(localPath)
	if err != nil {
		return "", fmt.Errorf("计算校验和失败: %w", err)
	}

	// 2. 创建备份记录，获取上传 URL
	createResp, err := t.createBackup()
	if err != nil {
		return "", fmt.Errorf("创建云备份记录失败: %w", err)
	}

	// 3. 上传文件
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("打开备份文件失败: %w", err)
	}
	defer file.Close()

	stat, _ := file.Stat()
	reader := &ProgressReader{
		Reader:   file,
		Total:    stat.Size(),
		Callback: progress,
	}

	req, err := http.NewRequest("PUT", createResp.UploadURL, reader)
	if err != nil {
		return "", fmt.Errorf("创建上传请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = stat.Size()

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("上传失败 (%d): %s", resp.StatusCode, string(body))
	}

	// 4. 确认上传完成（含校验和）
	if err := t.confirmBackup(createResp.BackupID, stat.Size(), remoteName, checksum); err != nil {
		return "", fmt.Errorf("确认备份失败: %w", err)
	}

	return fmt.Sprintf("cloud://%d", createResp.BackupID), nil
}

// Download 从云端下载备份文件
func (t *CloudTarget) Download(remotePath, localPath string, progress func(int)) error {
	// 从 remotePath 解析 backup ID
	var backupID int64
	fmt.Sscanf(remotePath, "cloud://%d", &backupID)
	if backupID == 0 {
		return fmt.Errorf("无效的云备份路径: %s", remotePath)
	}

	// 获取下载 URL
	resp, err := t.apiGet(fmt.Sprintf("/api/v1/cloud/backup/%d/download", backupID))
	if err != nil {
		return fmt.Errorf("获取下载地址失败: %w", err)
	}
	defer resp.Body.Close()

	var dlResp struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dlResp); err != nil {
		return fmt.Errorf("解析下载地址失败: %w", err)
	}

	// 下载文件
	dlRes, err := http.Get(dlResp.DownloadURL)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer dlRes.Body.Close()

	os.MkdirAll(filepath.Dir(localPath), 0o755)
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer file.Close()

	reader := &ProgressReader{
		Reader:   dlRes.Body,
		Total:    dlRes.ContentLength,
		Callback: progress,
	}

	_, err = io.Copy(file, reader)
	return err
}

// Delete 删除云端备份
func (t *CloudTarget) Delete(remotePath string) error {
	var backupID int64
	fmt.Sscanf(remotePath, "cloud://%d", &backupID)
	if backupID == 0 {
		return fmt.Errorf("无效的云备份路径: %s", remotePath)
	}

	req, err := http.NewRequest("DELETE",
		t.cfg.CloudURL+fmt.Sprintf("/api/v1/cloud/backup/%d", backupID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.cfg.CloudToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("删除失败: HTTP %d", resp.StatusCode)
	}
	return nil
}

// List 列出云端备份
func (t *CloudTarget) List() ([]RemoteFile, error) {
	resp, err := t.apiGet("/api/v1/cloud/backups")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Backups []struct {
			ID        int64  `json:"id"`
			SizeBytes int64  `json:"size_bytes"`
			Checksum  string `json:"checksum"`
			Version   int    `json:"version"`
			CreatedAt string `json:"created_at"`
		} `json:"backups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var files []RemoteFile
	for _, b := range result.Backups {
		t, _ := time.Parse(time.RFC3339, b.CreatedAt)
		files = append(files, RemoteFile{
			Name:    fmt.Sprintf("cloud-backup-v%d", b.Version),
			Path:    fmt.Sprintf("cloud://%d", b.ID),
			Size:    b.SizeBytes,
			ModTime: t.Unix(),
		})
	}
	return files, nil
}

// --- 内部方法 ---

type createBackupResponse struct {
	BackupID  int64  `json:"backup_id"`
	UploadURL string `json:"upload_url"`
}

func (t *CloudTarget) createBackup() (*createBackupResponse, error) {
	body, _ := json.Marshal(map[string]string{})
	resp, err := t.apiPost("/api/v1/cloud/backup", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result createBackupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (t *CloudTarget) confirmBackup(backupID, sizeBytes int64, filename, checksum string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"backup_id":  backupID,
		"size_bytes": sizeBytes,
		"checksum":   checksum,
		"metadata":   fmt.Sprintf(`{"filename":"%s"}`, filename),
	})
	resp, err := t.apiPost("/api/v1/cloud/backup/confirm", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *CloudTarget) apiGet(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", t.cfg.CloudURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.cfg.CloudToken)
	return t.client.Do(req)
}

func (t *CloudTarget) apiPost(path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", t.cfg.CloudURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.cfg.CloudToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("cloud API error (%d): %s", resp.StatusCode, string(respBody))
	}
	return resp, nil
}
