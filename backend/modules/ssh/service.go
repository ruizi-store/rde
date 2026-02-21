// Package ssh SSH远程连接模块 - 服务层实现
package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"github.com/ruizi-store/rde/backend/pkg/utils/encryption"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// Service SSH服务
type Service struct {
	db            *gorm.DB
	encryptionKey string
	logger        *zap.Logger

	// 会话管理
	sessions sync.Map // map[string]*Session

	// 传输队列
	transferTasks sync.Map              // map[string]*TransferTask
	transferQueue chan *TransferTask    // 传输任务队列
	transferStop  chan struct{}         // 停止信号
	progressChan  chan *TransferProgress // 进度通知
}

// NewService 创建服务实例
func NewService(db *gorm.DB, encryptionKey string, logger *zap.Logger) *Service {
	s := &Service{
		db:            db,
		encryptionKey: encryptionKey,
		logger:        logger,
		transferQueue: make(chan *TransferTask, 100),
		transferStop:  make(chan struct{}),
		progressChan:  make(chan *TransferProgress, 100),
	}

	// 启动传输工作协程
	for i := 0; i < TransferWorkers; i++ {
		go s.transferWorker()
	}

	return s
}

// Stop 停止服务，关闭所有会话
func (s *Service) Stop() {
	// 停止传输工作协程
	close(s.transferStop)

	s.sessions.Range(func(key, value interface{}) bool {
		if session, ok := value.(*Session); ok {
			s.closeSession(session)
		}
		return true
	})
}

// ==================== 连接配置管理 ====================

// CreateConnection 创建SSH连接配置
func (s *Service) CreateConnection(req *CreateConnectionRequest) (*Connection, error) {
	conn := &Connection{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		AuthMethod: req.AuthMethod,
	}

	if conn.Port == 0 {
		conn.Port = DefaultPort
	}

	// 加密凭据
	var credential string
	if req.AuthMethod == "password" {
		credential = req.Password
	} else {
		credential = req.PrivateKey
	}

	if credential != "" {
		encrypted, err := encryption.AESEncryptString(credential, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credential: %w", err)
		}
		conn.EncryptedCredential = encrypted
	}

	// 加密私钥密码
	if req.Passphrase != "" {
		encrypted, err := encryption.AESEncryptString(req.Passphrase, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt passphrase: %w", err)
		}
		conn.Passphrase = encrypted
	}

	if err := s.db.Create(conn).Error; err != nil {
		return nil, err
	}

	s.logger.Info("SSH connection created", zap.String("id", conn.ID), zap.String("name", conn.Name))
	return conn, nil
}

// UpdateConnection 更新SSH连接配置
func (s *Service) UpdateConnection(id string, req *UpdateConnectionRequest) (*Connection, error) {
	var conn Connection
	if err := s.db.First(&conn, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("connection not found")
		}
		return nil, err
	}

	// 更新字段
	if req.Name != "" {
		conn.Name = req.Name
	}
	if req.Host != "" {
		conn.Host = req.Host
	}
	if req.Port > 0 {
		conn.Port = req.Port
	}
	if req.Username != "" {
		conn.Username = req.Username
	}
	if req.AuthMethod != "" {
		conn.AuthMethod = req.AuthMethod
	}

	// 更新凭据
	if req.Password != "" || req.PrivateKey != "" {
		var credential string
		if conn.AuthMethod == "password" {
			credential = req.Password
		} else {
			credential = req.PrivateKey
		}
		encrypted, err := encryption.AESEncryptString(credential, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credential: %w", err)
		}
		conn.EncryptedCredential = encrypted
	}

	// 更新私钥密码
	if req.Passphrase != "" {
		encrypted, err := encryption.AESEncryptString(req.Passphrase, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt passphrase: %w", err)
		}
		conn.Passphrase = encrypted
	}

	if err := s.db.Save(&conn).Error; err != nil {
		return nil, err
	}

	return &conn, nil
}

// DeleteConnection 删除SSH连接配置
func (s *Service) DeleteConnection(id string) error {
	result := s.db.Delete(&Connection{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("connection not found")
	}
	s.logger.Info("SSH connection deleted", zap.String("id", id))
	return nil
}

// GetConnection 获取单个连接配置
func (s *Service) GetConnection(id string) (*Connection, error) {
	var conn Connection
	if err := s.db.First(&conn, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("connection not found")
		}
		return nil, err
	}
	return &conn, nil
}

// ListConnections 获取所有连接配置
func (s *Service) ListConnections() ([]Connection, error) {
	var connections []Connection
	if err := s.db.Order("last_used_at DESC, created_at DESC").Find(&connections).Error; err != nil {
		return nil, err
	}
	return connections, nil
}

// ==================== SSH会话管理 ====================

// Connect 建立SSH连接
func (s *Service) Connect(connID string, cols, rows uint16) (*Session, error) {
	// 获取连接配置
	conn, err := s.GetConnection(connID)
	if err != nil {
		return nil, err
	}

	// 解密凭据
	credential, err := encryption.AESDecryptString(conn.EncryptedCredential, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credential: %w", err)
	}

	var passphrase string
	if conn.Passphrase != "" {
		passphrase, err = encryption.AESDecryptString(conn.Passphrase, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt passphrase: %w", err)
		}
	}

	// 建立SSH连接
	client, err := s.dialSSH(conn.Host, conn.Port, conn.Username, conn.AuthMethod, credential, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// 创建会话
	session := &Session{
		ID:           uuid.New().String(),
		ConnectionID: connID,
		ConnName:     conn.Name,
		Host:         conn.Host,
		Username:     conn.Username,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		client:       client,
		stdin:        make(chan []byte, 256),
		done:         make(chan struct{}),
	}

	// 创建PTY会话
	if cols == 0 {
		cols = DefaultCols
	}
	if rows == 0 {
		rows = DefaultRows
	}

	ptySession, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// 请求PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := ptySession.RequestPty("xterm-256color", int(rows), int(cols), modes); err != nil {
		ptySession.Close()
		client.Close()
		return nil, fmt.Errorf("failed to request pty: %w", err)
	}

	session.ptySession = ptySession

	// 存储会话
	s.sessions.Store(session.ID, session)

	// 更新最后使用时间
	s.db.Model(&Connection{}).Where("id = ?", connID).Update("last_used_at", time.Now().Unix())

	s.logger.Info("SSH session created",
		zap.String("session_id", session.ID),
		zap.String("host", conn.Host))

	return session, nil
}

// dialSSH 建立SSH连接
func (s *Service) dialSSH(host string, port int, username, authMethod, credential, passphrase string) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	if authMethod == "password" {
		authMethods = append(authMethods, ssh.Password(credential))
	} else {
		// 解析私钥
		var signer ssh.Signer
		var err error
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(credential), []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(credential))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: 生产环境应验证host key
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// TestConnection 测试SSH连接
func (s *Service) TestConnection(req *TestConnectionRequest) error {
	port := req.Port
	if port == 0 {
		port = DefaultPort
	}

	client, err := s.dialSSH(req.Host, port, req.Username, req.AuthMethod, req.Password+req.PrivateKey, req.Passphrase)
	if err != nil {
		return err
	}
	client.Close()
	return nil
}

// GetSession 获取会话
func (s *Service) GetSession(sessionID string) (*Session, error) {
	value, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return value.(*Session), nil
}

// ListSessions 获取所有会话
func (s *Service) ListSessions() []*SessionInfo {
	var sessions []*SessionInfo
	s.sessions.Range(func(key, value interface{}) bool {
		if session, ok := value.(*Session); ok {
			sessions = append(sessions, session.ToInfo())
		}
		return true
	})
	return sessions
}

// CloseSession 关闭会话
func (s *Service) CloseSession(sessionID string) error {
	value, ok := s.sessions.LoadAndDelete(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	session := value.(*Session)
	s.closeSession(session)
	return nil
}

// closeSession 内部关闭会话
func (s *Service) closeSession(session *Session) {
	session.mu.Lock()
	defer session.mu.Unlock()

	// 关闭done channel
	select {
	case <-session.done:
	default:
		close(session.done)
	}

	// 关闭SFTP客户端
	if session.sftpClient != nil {
		session.sftpClient.Close()
	}

	// 关闭PTY会话
	if session.ptySession != nil {
		session.ptySession.Close()
	}

	// 关闭SSH客户端
	if session.client != nil {
		session.client.Close()
	}

	// 关闭WebSocket
	if session.ws != nil {
		session.ws.Close()
	}

	s.logger.Info("SSH session closed", zap.String("session_id", session.ID))
}

// ResizeTerminal 调整终端大小
func (s *Service) ResizeTerminal(sessionID string, cols, rows uint16) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.ptySession == nil {
		return fmt.Errorf("no pty session")
	}

	return session.ptySession.WindowChange(int(rows), int(cols))
}

// ==================== SFTP操作 ====================

// getSftpClient 获取或创建SFTP客户端
func (s *Service) getSftpClient(session *Session) (*sftp.Client, error) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.sftpClient != nil {
		return session.sftpClient, nil
	}

	client, err := sftp.NewClient(session.client)
	if err != nil {
		return nil, err
	}
	session.sftpClient = client
	return client, nil
}

// ListDir 列出目录内容
func (s *Service) ListDir(sessionID, path string) ([]FileInfo, error) {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return nil, fmt.Errorf("failed to create sftp client: %w", err)
	}

	entries, err := sftpClient.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		info := FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    entry.Size(),
			Mode:    entry.Mode().String(),
			ModTime: entry.ModTime().Unix(),
			IsDir:   entry.IsDir(),
			IsLink:  entry.Mode()&os.ModeSymlink != 0,
		}
		files = append(files, info)
	}

	return files, nil
}

// Stat 获取文件信息
func (s *Service) Stat(sessionID, path string) (*FileInfo, error) {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return nil, fmt.Errorf("failed to create sftp client: %w", err)
	}

	info, err := sftpClient.Stat(path)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Name:    info.Name(),
		Path:    path,
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().Unix(),
		IsDir:   info.IsDir(),
		IsLink:  info.Mode()&os.ModeSymlink != 0,
	}, nil
}

// Mkdir 创建目录
func (s *Service) Mkdir(sessionID, path string) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}

	return sftpClient.MkdirAll(path)
}

// Rename 重命名/移动文件
func (s *Service) Rename(sessionID, oldPath, newPath string) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}

	return sftpClient.Rename(oldPath, newPath)
}

// Delete 删除文件或目录
func (s *Service) Delete(sessionID string, paths []string) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}

	for _, path := range paths {
		info, err := sftpClient.Stat(path)
		if err != nil {
			continue
		}

		if info.IsDir() {
			if err := s.removeAllRemote(sftpClient, path); err != nil {
				return fmt.Errorf("failed to remove %s: %w", path, err)
			}
		} else {
			if err := sftpClient.Remove(path); err != nil {
				return fmt.Errorf("failed to remove %s: %w", path, err)
			}
		}
	}

	return nil
}

// removeAllRemote 递归删除远程目录
func (s *Service) removeAllRemote(client *sftp.Client, path string) error {
	entries, err := client.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if err := s.removeAllRemote(client, fullPath); err != nil {
				return err
			}
		} else {
			if err := client.Remove(fullPath); err != nil {
				return err
			}
		}
	}

	return client.RemoveDirectory(path)
}

// ==================== 文件传输 ====================

// DownloadFile 下载单个文件
func (s *Service) DownloadFile(sessionID, remotePath, localPath string, progressChan chan<- int64) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}

	// 打开远程文件
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// 确保本地目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// 创建本地文件
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// 复制数据
	if progressChan != nil {
		_, err = io.Copy(localFile, &progressReader{reader: remoteFile, progress: progressChan})
	} else {
		_, err = io.Copy(localFile, remoteFile)
	}

	return err
}

// UploadFile 上传单个文件
func (s *Service) UploadFile(sessionID, localPath, remotePath string, progressChan chan<- int64) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}

	// 打开本地文件
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// 确保远程目录存在
	remoteDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// 创建远程文件
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// 复制数据
	if progressChan != nil {
		_, err = io.Copy(remoteFile, &progressReader{reader: localFile, progress: progressChan})
	} else {
		_, err = io.Copy(remoteFile, localFile)
	}

	return err
}

// UploadFromReader 从Reader上传文件
func (s *Service) UploadFromReader(sessionID, remotePath string, reader io.Reader, size int64) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}

	// 确保远程目录存在
	remoteDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// 创建远程文件
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// 复制数据
	_, err = io.Copy(remoteFile, reader)
	return err
}

// progressReader 带进度回调的Reader
type progressReader struct {
	reader   io.Reader
	progress chan<- int64
	total    int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.total += int64(n)
		select {
		case pr.progress <- pr.total:
		default:
		}
	}
	return n, err
}

// ==================== 连接测试工具 ====================

// CheckPort 检查端口是否可达
func (s *Service) CheckPort(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// ==================== 传输队列管理 ====================

// CreateTransferTask 创建传输任务
func (s *Service) CreateTransferTask(sessionID, taskType, localPath, remotePath, fileName string, size int64) (*TransferTask, error) {
	task := &TransferTask{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		Type:       taskType,
		LocalPath:  localPath,
		RemotePath: remotePath,
		FileName:   fileName,
		Size:       size,
		Status:     "pending",
		CreatedAt:  time.Now().Unix(),
	}

	s.transferTasks.Store(task.ID, task)

	// 放入队列
	select {
	case s.transferQueue <- task:
	default:
		task.Status = "failed"
		task.Error = "传输队列已满"
		return task, fmt.Errorf("transfer queue is full")
	}

	return task, nil
}

// GetTransferTask 获取传输任务
func (s *Service) GetTransferTask(taskID string) (*TransferTask, error) {
	if value, ok := s.transferTasks.Load(taskID); ok {
		return value.(*TransferTask), nil
	}
	return nil, fmt.Errorf("task not found")
}

// ListTransferTasks 列出所有传输任务
func (s *Service) ListTransferTasks(sessionID string) []*TransferTask {
	var tasks []*TransferTask
	s.transferTasks.Range(func(key, value interface{}) bool {
		task := value.(*TransferTask)
		if sessionID == "" || task.SessionID == sessionID {
			tasks = append(tasks, task)
		}
		return true
	})
	return tasks
}

// CancelTransferTask 取消传输任务
func (s *Service) CancelTransferTask(taskID string) error {
	value, ok := s.transferTasks.Load(taskID)
	if !ok {
		return fmt.Errorf("task not found")
	}

	task := value.(*TransferTask)
	if task.Status == "done" || task.Status == "failed" || task.Status == "cancelled" {
		return fmt.Errorf("cannot cancel completed task")
	}

	task.Status = "cancelled"
	task.FinishedAt = time.Now().Unix()
	return nil
}

// ClearCompletedTasks 清除已完成的任务
func (s *Service) ClearCompletedTasks() {
	s.transferTasks.Range(func(key, value interface{}) bool {
		task := value.(*TransferTask)
		if task.Status == "done" || task.Status == "failed" || task.Status == "cancelled" {
			s.transferTasks.Delete(key)
		}
		return true
	})
}

// GetProgressChannel 获取进度通知通道
func (s *Service) GetProgressChannel() <-chan *TransferProgress {
	return s.progressChan
}

// transferWorker 传输工作协程
func (s *Service) transferWorker() {
	for {
		select {
		case <-s.transferStop:
			return
		case task := <-s.transferQueue:
			if task.Status == "cancelled" {
				continue
			}

			s.executeTransfer(task)
		}
	}
}

// executeTransfer 执行传输任务
func (s *Service) executeTransfer(task *TransferTask) {
	task.Status = "running"
	task.StartedAt = time.Now().Unix()

	startTime := time.Now()

	// 发送初始进度
	s.sendProgress(task, 0)

	// 获取会话
	session, err := s.GetSession(task.SessionID)
	if err != nil {
		task.Status = "failed"
		task.Error = "会话不存在: " + err.Error()
		task.FinishedAt = time.Now().Unix()
		s.sendProgress(task, 0)
		return
	}

	// 获取 SFTP 客户端
	sftpClient, err := s.getSftpClient(session)
	if err != nil {
		task.Status = "failed"
		task.Error = "SFTP连接失败: " + err.Error()
		task.FinishedAt = time.Now().Unix()
		s.sendProgress(task, 0)
		return
	}

	var transferred int64

	switch task.Type {
	case "download":
		transferred, err = s.doDownload(sftpClient, task)
	case "upload":
		transferred, err = s.doUpload(sftpClient, task)
	}

	if err != nil {
		if task.Status != "cancelled" {
			task.Status = "failed"
			task.Error = err.Error()
		}
	} else if task.Status != "cancelled" {
		task.Status = "done"
		task.Transferred = transferred
	}

	task.FinishedAt = time.Now().Unix()

	// 计算速度
	elapsed := time.Since(startTime).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(transferred) / elapsed
	}

	// 发送最终进度
	s.progressChan <- &TransferProgress{
		TaskID:      task.ID,
		FileName:    task.FileName,
		Type:        task.Type,
		Size:        task.Size,
		Transferred: transferred,
		Speed:       speed,
		Status:      task.Status,
		Error:       task.Error,
	}
}

// doDownload 执行下载
func (s *Service) doDownload(sftpClient *sftp.Client, task *TransferTask) (int64, error) {
	// 打开远程文件
	remoteFile, err := sftpClient.Open(task.RemotePath)
	if err != nil {
		return 0, fmt.Errorf("打开远程文件失败: %w", err)
	}
	defer remoteFile.Close()

	// 创建本地文件
	localFile, err := os.Create(task.LocalPath)
	if err != nil {
		return 0, fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer localFile.Close()

	// 复制数据并报告进度
	buf := make([]byte, 32*1024)
	var total int64
	lastReport := time.Now()

	for {
		if task.Status == "cancelled" {
			return total, fmt.Errorf("任务已取消")
		}

		n, readErr := remoteFile.Read(buf)
		if n > 0 {
			_, writeErr := localFile.Write(buf[:n])
			if writeErr != nil {
				return total, fmt.Errorf("写入本地文件失败: %w", writeErr)
			}
			total += int64(n)
			task.Transferred = total

			// 每100ms报告一次进度
			if time.Since(lastReport) > 100*time.Millisecond {
				s.sendProgress(task, total)
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return total, fmt.Errorf("读取远程文件失败: %w", readErr)
		}
	}

	return total, nil
}

// doUpload 执行上传
func (s *Service) doUpload(sftpClient *sftp.Client, task *TransferTask) (int64, error) {
	// 打开本地文件
	localFile, err := os.Open(task.LocalPath)
	if err != nil {
		return 0, fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer localFile.Close()

	// 确保远程目录存在
	remoteDir := filepath.Dir(task.RemotePath)
	sftpClient.MkdirAll(remoteDir)

	// 创建远程文件
	remoteFile, err := sftpClient.Create(task.RemotePath)
	if err != nil {
		return 0, fmt.Errorf("创建远程文件失败: %w", err)
	}
	defer remoteFile.Close()

	// 复制数据并报告进度
	buf := make([]byte, 32*1024)
	var total int64
	lastReport := time.Now()

	for {
		if task.Status == "cancelled" {
			return total, fmt.Errorf("任务已取消")
		}

		n, readErr := localFile.Read(buf)
		if n > 0 {
			_, writeErr := remoteFile.Write(buf[:n])
			if writeErr != nil {
				return total, fmt.Errorf("写入远程文件失败: %w", writeErr)
			}
			total += int64(n)
			task.Transferred = total

			// 每100ms报告一次进度
			if time.Since(lastReport) > 100*time.Millisecond {
				s.sendProgress(task, total)
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return total, fmt.Errorf("读取本地文件失败: %w", readErr)
		}
	}

	return total, nil
}

// sendProgress 发送进度
func (s *Service) sendProgress(task *TransferTask, transferred int64) {
	select {
	case s.progressChan <- &TransferProgress{
		TaskID:      task.ID,
		FileName:    task.FileName,
		Type:        task.Type,
		Size:        task.Size,
		Transferred: transferred,
		Status:      task.Status,
	}:
	default:
	}
}
