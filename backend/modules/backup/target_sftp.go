package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPTarget SFTP 存储目标
type SFTPTarget struct {
	config     SFTPTargetConfig
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// Configure 配置 SFTP 目标
func (t *SFTPTarget) Configure(config string) error {
	if err := json.Unmarshal([]byte(config), &t.config); err != nil {
		return fmt.Errorf("解析 SFTP 配置失败: %w", err)
	}

	if t.config.Host == "" {
		return fmt.Errorf("SFTP Host 不能为空")
	}
	if t.config.Username == "" {
		return fmt.Errorf("SFTP Username 不能为空")
	}
	if t.config.Port == 0 {
		t.config.Port = 22
	}

	return nil
}

// connect 建立 SFTP 连接
func (t *SFTPTarget) connect() error {
	if t.sftpClient != nil {
		return nil
	}

	var authMethods []ssh.AuthMethod

	// 密码认证
	if t.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(t.config.Password))
	}

	// 私钥认证
	if t.config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(t.config.PrivateKey))
		if err != nil {
			return fmt.Errorf("解析私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("需要提供密码或私钥")
	}

	sshConfig := &ssh.ClientConfig{
		User:            t.config.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: 生产环境应该验证主机密钥
	}

	addr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	t.sshClient = sshClient

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("SFTP 连接失败: %w", err)
	}
	t.sftpClient = sftpClient

	return nil
}

// close 关闭连接
func (t *SFTPTarget) close() {
	if t.sftpClient != nil {
		t.sftpClient.Close()
		t.sftpClient = nil
	}
	if t.sshClient != nil {
		t.sshClient.Close()
		t.sshClient = nil
	}
}

// Test 测试 SFTP 连接
func (t *SFTPTarget) Test() *TargetTestResponse {
	if err := t.connect(); err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: err.Error(),
		}
	}
	defer t.close()

	// 检查/创建备份目录
	backupPath := t.getBackupPath()
	if err := t.sftpClient.MkdirAll(backupPath); err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "创建备份目录失败: " + err.Error(),
		}
	}

	// 获取可用空间（如果支持）
	stat, err := t.sftpClient.StatVFS(backupPath)
	var freeSpace int64
	if err == nil {
		freeSpace = int64(stat.Bavail * stat.Bsize)
	}

	return &TargetTestResponse{
		Success:   true,
		Message:   "连接成功",
		FreeSpace: freeSpace,
	}
}

// Upload 上传文件到 SFTP
func (t *SFTPTarget) Upload(localPath, remoteName string, progress func(int)) (string, error) {
	if err := t.connect(); err != nil {
		return "", err
	}
	defer t.close()

	// 打开本地文件
	localFile, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 确保远程目录存在
	backupPath := t.getBackupPath()
	if err := t.sftpClient.MkdirAll(backupPath); err != nil {
		return "", fmt.Errorf("创建远程目录失败: %w", err)
	}

	remotePath := path.Join(backupPath, remoteName)

	// 创建远程文件
	remoteFile, err := t.sftpClient.Create(remotePath)
	if err != nil {
		return "", fmt.Errorf("创建远程文件失败: %w", err)
	}
	defer remoteFile.Close()

	// 带进度上传
	reader := &ProgressReader{
		Reader:   localFile,
		Total:    stat.Size(),
		Callback: progress,
	}

	_, err = io.Copy(remoteFile, reader)
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	return remotePath, nil
}

// Download 从 SFTP 下载文件
func (t *SFTPTarget) Download(remotePath, localPath string, progress func(int)) error {
	if err := t.connect(); err != nil {
		return err
	}
	defer t.close()

	// 获取远程文件信息
	stat, err := t.sftpClient.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("获取远程文件信息失败: %w", err)
	}

	// 打开远程文件
	remoteFile, err := t.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("打开远程文件失败: %w", err)
	}
	defer remoteFile.Close()

	// 确保本地目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	// 创建本地文件
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer localFile.Close()

	// 带进度下载
	reader := &ProgressReader{
		Reader:   remoteFile,
		Total:    stat.Size(),
		Callback: progress,
	}

	_, err = io.Copy(localFile, reader)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	return nil
}

// Delete 删除 SFTP 文件
func (t *SFTPTarget) Delete(remotePath string) error {
	if err := t.connect(); err != nil {
		return err
	}
	defer t.close()

	return t.sftpClient.Remove(remotePath)
}

// List 列出备份文件
func (t *SFTPTarget) List() ([]RemoteFile, error) {
	if err := t.connect(); err != nil {
		return nil, err
	}
	defer t.close()

	backupPath := t.getBackupPath()
	files, err := t.sftpClient.ReadDir(backupPath)
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
func (t *SFTPTarget) getBackupPath() string {
	p := t.config.Path
	if p == "" {
		p = "/home/" + t.config.Username + "/rde-backups"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
