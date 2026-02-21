// Package backup 提供备份还原功能
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// 配对码相关常量
const (
	PairCodeLength   = 9            // ABC-123-XYZ 格式
	PairCodeExpiry   = 10 * time.Minute
	ChunkSize        = 4 * 1024 * 1024 // 4MB 分块
	MaxConcurrentTx  = 3               // 最大并发传输数
	HeartbeatInterval = 30 * time.Second
)

// MigrateSessionRole 会话角色
type MigrateSessionRole string

const (
	RoleSource MigrateSessionRole = "source" // 发送方
	RoleTarget MigrateSessionRole = "target" // 接收方
)

// MigrateSessionStatus 会话状态
type MigrateSessionStatus string

const (
	StatusPairing      MigrateSessionStatus = "pairing"
	StatusConnected    MigrateSessionStatus = "connected"
	StatusTransferring MigrateSessionStatus = "transferring"
	StatusCompleted    MigrateSessionStatus = "completed"
	StatusFailed       MigrateSessionStatus = "failed"
	StatusCancelled    MigrateSessionStatus = "cancelled"
)

// MigrateSession 迁移会话
type MigrateSession struct {
	ID            string               `json:"id"`
	PairCode      string               `json:"pair_code"`
	Role          MigrateSessionRole   `json:"role"`
	Status        MigrateSessionStatus `json:"status"`
	RemoteAddr    string               `json:"remote_addr,omitempty"`
	RemoteHost    string               `json:"remote_host,omitempty"`
	ExpiresAt     time.Time            `json:"expires_at"`
	CreatedAt     time.Time            `json:"created_at"`
	
	// 内部字段
	privateKey    *ecdh.PrivateKey     `json:"-"`
	sharedSecret  []byte               `json:"-"`
	conn          *websocket.Conn      `json:"-"`
	content       *MigrateContentSelection `json:"-"`
	progress      *MigrateProgress     `json:"-"`
	mu            sync.RWMutex         `json:"-"`
}

// MigrateContentSelection 迁移内容选择
type MigrateContentSelection struct {
	SystemConfig bool     `json:"system_config"`
	Users        bool     `json:"users"`
	Docker       bool     `json:"docker"`
	Network      bool     `json:"network"`
	Samba        bool     `json:"samba"`
	Files        []string `json:"files"`
	Apps         []string `json:"apps"`
}

// MigrateProgress 迁移进度
type MigrateProgress struct {
	Phase           string  `json:"phase"` // preparing, config, files, finalizing
	TotalSize       int64   `json:"total_size"`
	TransferredSize int64   `json:"transferred_size"`
	TotalFiles      int     `json:"total_files"`
	TransferredFiles int    `json:"transferred_files"`
	CurrentFile     string  `json:"current_file"`
	Speed           int64   `json:"speed"` // bytes/sec
	ETA             int     `json:"eta"`   // 预计剩余秒数
	Error           string  `json:"error,omitempty"`
}

// P2P 消息类型
type MsgType string

const (
	MsgTypeHandshake       MsgType = "handshake"
	MsgTypeHandshakeAck    MsgType = "handshake_ack"
	MsgTypeContentList     MsgType = "content_list"
	MsgTypeContentSelect   MsgType = "content_select"
	MsgTypeTransferStart   MsgType = "transfer_start"
	MsgTypeChunk           MsgType = "chunk"
	MsgTypeChunkAck        MsgType = "chunk_ack"
	MsgTypeTransferDone    MsgType = "transfer_done"
	MsgTypeError           MsgType = "error"
	MsgTypeHeartbeat       MsgType = "heartbeat"
	MsgTypeCancel          MsgType = "cancel"
)

// P2PMessage P2P 消息
type P2PMessage struct {
	Type    MsgType         `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// HandshakePayload 握手消息
type HandshakePayload struct {
	PublicKey string `json:"public_key"`
	Hostname  string `json:"hostname"`
	Version   string `json:"version"`
}

// ChunkPayload 数据块消息
type ChunkPayload struct {
	FileID    string `json:"file_id"`
	FilePath  string `json:"file_path"`
	ChunkIdx  int    `json:"chunk_idx"`
	TotalChunks int  `json:"total_chunks"`
	Data      string `json:"data"` // base64 编码的加密数据
	Checksum  string `json:"checksum"`
	IsLast    bool   `json:"is_last"`
}

// ChunkAckPayload 数据块确认消息
type ChunkAckPayload struct {
	FileID   string `json:"file_id"`
	ChunkIdx int    `json:"chunk_idx"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// P2PMigrateService P2P 迁移服务
type P2PMigrateService struct {
	sessions    map[string]*MigrateSession
	pairCodes   map[string]string // pairCode -> sessionID
	mu          sync.RWMutex
	dataDir     string
	service     *Service
	upgrader    websocket.Upgrader
}

// NewP2PMigrateService 创建 P2P 迁移服务
func NewP2PMigrateService(service *Service) *P2PMigrateService {
	return &P2PMigrateService{
		sessions:  make(map[string]*MigrateSession),
		pairCodes: make(map[string]string),
		dataDir:   service.dataDir,
		service:   service,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许跨域
			},
		},
	}
}

// GeneratePairCode 生成配对码（目标端调用）
func (p *P2PMigrateService) GeneratePairCode() (*MigrateSession, error) {
	// 生成会话 ID
	sessionID := generateID()
	
	// 生成配对码
	pairCode := generatePairCode()
	
	// 生成 ECDH 密钥对
	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}
	
	session := &MigrateSession{
		ID:         sessionID,
		PairCode:   pairCode,
		Role:       RoleTarget,
		Status:     StatusPairing,
		ExpiresAt:  time.Now().Add(PairCodeExpiry),
		CreatedAt:  time.Now(),
		privateKey: privateKey,
		progress:   &MigrateProgress{Phase: "waiting"},
	}
	
	p.mu.Lock()
	p.sessions[sessionID] = session
	p.pairCodes[pairCode] = sessionID
	p.mu.Unlock()
	
	// 启动过期清理
	go p.cleanupExpiredSession(sessionID, PairCodeExpiry)
	
	return session, nil
}

// ConnectWithPairCode 使用配对码连接（源端调用）
func (p *P2PMigrateService) ConnectWithPairCode(pairCode, targetURL string) (*MigrateSession, error) {
	// 生成本地会话
	sessionID := generateID()
	
	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}
	
	session := &MigrateSession{
		ID:         sessionID,
		PairCode:   pairCode,
		Role:       RoleSource,
		Status:     StatusPairing,
		RemoteAddr: targetURL,
		CreatedAt:  time.Now(),
		privateKey: privateKey,
		progress:   &MigrateProgress{Phase: "connecting"},
	}
	
	p.mu.Lock()
	p.sessions[sessionID] = session
	p.mu.Unlock()
	
	return session, nil
}

// ValidatePairCode 验证配对码
func (p *P2PMigrateService) ValidatePairCode(pairCode string) (*MigrateSession, error) {
	p.mu.RLock()
	sessionID, exists := p.pairCodes[pairCode]
	p.mu.RUnlock()
	
	if !exists {
		return nil, errors.New("无效的配对码")
	}
	
	p.mu.RLock()
	session, exists := p.sessions[sessionID]
	p.mu.RUnlock()
	
	if !exists {
		return nil, errors.New("会话不存在")
	}
	
	if time.Now().After(session.ExpiresAt) {
		return nil, errors.New("配对码已过期")
	}
	
	if session.Status != StatusPairing {
		return nil, errors.New("会话状态无效")
	}
	
	return session, nil
}

// GetSession 获取会话
func (p *P2PMigrateService) GetSession(sessionID string) (*MigrateSession, error) {
	p.mu.RLock()
	session, exists := p.sessions[sessionID]
	p.mu.RUnlock()
	
	if !exists {
		return nil, errors.New("会话不存在")
	}
	
	return session, nil
}

// GetSessionByPairCode 通过配对码获取会话
func (p *P2PMigrateService) GetSessionByPairCode(pairCode string) (*MigrateSession, error) {
	p.mu.RLock()
	sessionID, exists := p.pairCodes[pairCode]
	p.mu.RUnlock()
	
	if !exists {
		return nil, errors.New("配对码无效")
	}
	
	return p.GetSession(sessionID)
}

// GetProgress 获取迁移进度
func (p *P2PMigrateService) GetProgress(sessionID string) (*MigrateProgress, error) {
	session, err := p.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	
	session.mu.RLock()
	defer session.mu.RUnlock()
	
	if session.progress == nil {
		return &MigrateProgress{Phase: "unknown"}, nil
	}
	
	return session.progress, nil
}

// CancelSession 取消会话
func (p *P2PMigrateService) CancelSession(sessionID string) error {
	session, err := p.GetSession(sessionID)
	if err != nil {
		return err
	}
	
	session.mu.Lock()
	session.Status = StatusCancelled
	if session.conn != nil {
		msg := P2PMessage{Type: MsgTypeCancel}
		_ = session.conn.WriteJSON(msg)
		session.conn.Close()
	}
	session.mu.Unlock()
	
	p.cleanupSession(sessionID)
	return nil
}

// HandleWebSocket 处理 WebSocket 连接
func (p *P2PMigrateService) HandleWebSocket(w http.ResponseWriter, r *http.Request, pairCode string) error {
	session, err := p.ValidatePairCode(pairCode)
	if err != nil {
		return err
	}
	
	conn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("WebSocket 升级失败: %w", err)
	}
	
	session.mu.Lock()
	session.conn = conn
	session.RemoteAddr = r.RemoteAddr
	session.mu.Unlock()
	
	// 处理连接
	go p.handleConnection(session)
	
	return nil
}

// handleConnection 处理 P2P 连接
func (p *P2PMigrateService) handleConnection(session *MigrateSession) {
	defer func() {
		session.mu.Lock()
		if session.conn != nil {
			session.conn.Close()
		}
		session.mu.Unlock()
	}()
	
	// 发送握手消息
	pubKeyBytes := session.privateKey.PublicKey().Bytes()
	handshake := HandshakePayload{
		PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes),
		Hostname:  hostname(),
		Version:   "1.0",
	}
	payload, _ := json.Marshal(handshake)
	
	msg := P2PMessage{
		Type:    MsgTypeHandshake,
		Payload: payload,
	}
	
	session.mu.RLock()
	conn := session.conn
	session.mu.RUnlock()
	
	if err := conn.WriteJSON(msg); err != nil {
		p.setSessionError(session, "发送握手失败: "+err.Error())
		return
	}
	
	// 启动心跳
	go p.heartbeatLoop(session)
	
	// 消息处理循环
	for {
		var msg P2PMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				break
			}
			p.setSessionError(session, "读取消息失败: "+err.Error())
			break
		}
		
		if err := p.handleMessage(session, &msg); err != nil {
			p.setSessionError(session, err.Error())
			break
		}
	}
}

// handleMessage 处理 P2P 消息
func (p *P2PMigrateService) handleMessage(session *MigrateSession, msg *P2PMessage) error {
	switch msg.Type {
	case MsgTypeHandshake:
		return p.handleHandshake(session, msg.Payload)
	case MsgTypeHandshakeAck:
		return p.handleHandshakeAck(session, msg.Payload)
	case MsgTypeContentList:
		return p.handleContentList(session, msg.Payload)
	case MsgTypeContentSelect:
		return p.handleContentSelect(session, msg.Payload)
	case MsgTypeTransferStart:
		return p.handleTransferStart(session, msg.Payload)
	case MsgTypeChunk:
		return p.handleChunk(session, msg.Payload)
	case MsgTypeChunkAck:
		return p.handleChunkAck(session, msg.Payload)
	case MsgTypeTransferDone:
		return p.handleTransferDone(session)
	case MsgTypeCancel:
		session.mu.Lock()
		session.Status = StatusCancelled
		session.mu.Unlock()
		return errors.New("对方取消了迁移")
	case MsgTypeHeartbeat:
		// 忽略心跳
		return nil
	default:
		return fmt.Errorf("未知消息类型: %s", msg.Type)
	}
}

// handleHandshake 处理握手消息
func (p *P2PMigrateService) handleHandshake(session *MigrateSession, payload json.RawMessage) error {
	var hs HandshakePayload
	if err := json.Unmarshal(payload, &hs); err != nil {
		return err
	}
	
	// 解析对方公钥
	pubKeyBytes, err := base64.StdEncoding.DecodeString(hs.PublicKey)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %w", err)
	}
	
	remotePubKey, err := ecdh.P256().NewPublicKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("创建公钥失败: %w", err)
	}
	
	// 计算共享密钥
	sharedSecret, err := session.privateKey.ECDH(remotePubKey)
	if err != nil {
		return fmt.Errorf("密钥协商失败: %w", err)
	}
	
	session.mu.Lock()
	session.sharedSecret = sharedSecret
	session.RemoteHost = hs.Hostname
	session.Status = StatusConnected
	session.mu.Unlock()
	
	// 发送握手确认
	myPubKey := session.privateKey.PublicKey().Bytes()
	ack := HandshakePayload{
		PublicKey: base64.StdEncoding.EncodeToString(myPubKey),
		Hostname:  hostname(),
		Version:   "1.0",
	}
	ackPayload, _ := json.Marshal(ack)
	
	return session.conn.WriteJSON(P2PMessage{
		Type:    MsgTypeHandshakeAck,
		Payload: ackPayload,
	})
}

// handleHandshakeAck 处理握手确认
func (p *P2PMigrateService) handleHandshakeAck(session *MigrateSession, payload json.RawMessage) error {
	var hs HandshakePayload
	if err := json.Unmarshal(payload, &hs); err != nil {
		return err
	}
	
	pubKeyBytes, err := base64.StdEncoding.DecodeString(hs.PublicKey)
	if err != nil {
		return err
	}
	
	remotePubKey, err := ecdh.P256().NewPublicKey(pubKeyBytes)
	if err != nil {
		return err
	}
	
	sharedSecret, err := session.privateKey.ECDH(remotePubKey)
	if err != nil {
		return err
	}
	
	session.mu.Lock()
	session.sharedSecret = sharedSecret
	session.RemoteHost = hs.Hostname
	session.Status = StatusConnected
	session.mu.Unlock()
	
	return nil
}

// handleContentList 处理可迁移内容列表
func (p *P2PMigrateService) handleContentList(session *MigrateSession, payload json.RawMessage) error {
	// 目标端收到源端的可迁移内容列表
	// TODO: 向前端推送可选择的内容
	return nil
}

// handleContentSelect 处理内容选择
func (p *P2PMigrateService) handleContentSelect(session *MigrateSession, payload json.RawMessage) error {
	var content MigrateContentSelection
	if err := json.Unmarshal(payload, &content); err != nil {
		return err
	}
	
	session.mu.Lock()
	session.content = &content
	session.mu.Unlock()
	
	return nil
}

// handleTransferStart 处理传输开始
func (p *P2PMigrateService) handleTransferStart(session *MigrateSession, payload json.RawMessage) error {
	session.mu.Lock()
	session.Status = StatusTransferring
	session.progress = &MigrateProgress{Phase: "transferring"}
	session.mu.Unlock()
	return nil
}

// handleChunk 处理数据块
func (p *P2PMigrateService) handleChunk(session *MigrateSession, payload json.RawMessage) error {
	var chunk ChunkPayload
	if err := json.Unmarshal(payload, &chunk); err != nil {
		return err
	}
	
	// 解密数据
	encData, err := base64.StdEncoding.DecodeString(chunk.Data)
	if err != nil {
		return p.sendChunkAck(session, chunk.FileID, chunk.ChunkIdx, false, "解码失败")
	}
	
	data, err := p.decryptData(session.sharedSecret, encData)
	if err != nil {
		return p.sendChunkAck(session, chunk.FileID, chunk.ChunkIdx, false, "解密失败")
	}
	
	// 验证校验和
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	if checksum != chunk.Checksum {
		return p.sendChunkAck(session, chunk.FileID, chunk.ChunkIdx, false, "校验和不匹配")
	}
	
	// 写入文件
	targetPath := filepath.Join(p.dataDir, "migrate-temp", chunk.FilePath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return p.sendChunkAck(session, chunk.FileID, chunk.ChunkIdx, false, "创建目录失败")
	}
	
	flags := os.O_CREATE | os.O_WRONLY
	if chunk.ChunkIdx == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	
	f, err := os.OpenFile(targetPath, flags, 0644)
	if err != nil {
		return p.sendChunkAck(session, chunk.FileID, chunk.ChunkIdx, false, "打开文件失败")
	}
	defer f.Close()
	
	if _, err := f.Write(data); err != nil {
		return p.sendChunkAck(session, chunk.FileID, chunk.ChunkIdx, false, "写入失败")
	}
	
	// 更新进度
	session.mu.Lock()
	session.progress.TransferredSize += int64(len(data))
	if chunk.IsLast {
		session.progress.TransferredFiles++
	}
	session.progress.CurrentFile = chunk.FilePath
	session.mu.Unlock()
	
	return p.sendChunkAck(session, chunk.FileID, chunk.ChunkIdx, true, "")
}

// handleChunkAck 处理数据块确认
func (p *P2PMigrateService) handleChunkAck(session *MigrateSession, payload json.RawMessage) error {
	var ack ChunkAckPayload
	if err := json.Unmarshal(payload, &ack); err != nil {
		return err
	}
	
	if !ack.Success {
		return fmt.Errorf("传输失败 [%s:%d]: %s", ack.FileID, ack.ChunkIdx, ack.Error)
	}
	
	return nil
}

// handleTransferDone 处理传输完成
func (p *P2PMigrateService) handleTransferDone(session *MigrateSession) error {
	session.mu.Lock()
	session.Status = StatusCompleted
	session.progress.Phase = "completed"
	session.mu.Unlock()
	
	// 应用迁移的配置
	go p.applyMigratedContent(session)
	
	return nil
}

// sendChunkAck 发送数据块确认
func (p *P2PMigrateService) sendChunkAck(session *MigrateSession, fileID string, chunkIdx int, success bool, errMsg string) error {
	ack := ChunkAckPayload{
		FileID:   fileID,
		ChunkIdx: chunkIdx,
		Success:  success,
		Error:    errMsg,
	}
	payload, _ := json.Marshal(ack)
	
	return session.conn.WriteJSON(P2PMessage{
		Type:    MsgTypeChunkAck,
		Payload: payload,
	})
}

// StartTransfer 开始传输（源端调用）
func (p *P2PMigrateService) StartTransfer(sessionID string, content *MigrateContentSelection) error {
	session, err := p.GetSession(sessionID)
	if err != nil {
		return err
	}
	
	if session.Status != StatusConnected {
		return errors.New("会话未连接")
	}
	
	session.mu.Lock()
	session.content = content
	session.Status = StatusTransferring
	session.progress = &MigrateProgress{Phase: "preparing"}
	session.mu.Unlock()
	
	// 发送传输开始消息
	if err := session.conn.WriteJSON(P2PMessage{Type: MsgTypeTransferStart}); err != nil {
		return err
	}
	
	// 异步执行传输
	go p.executeTransfer(session)
	
	return nil
}

// executeTransfer 执行传输
func (p *P2PMigrateService) executeTransfer(session *MigrateSession) {
	session.mu.RLock()
	content := session.content
	session.mu.RUnlock()
	
	// 收集要传输的文件
	var files []transferFile
	
	if content.SystemConfig {
		files = append(files, p.collectConfigFiles()...)
	}
	if content.Docker {
		files = append(files, p.collectDockerFiles()...)
	}
	if content.Samba {
		files = append(files, p.collectSambaFiles()...)
	}
	for _, path := range content.Files {
		files = append(files, p.collectPathFiles(path)...)
	}
	
	// 计算总大小
	var totalSize int64
	for _, f := range files {
		totalSize += f.size
	}
	
	session.mu.Lock()
	session.progress.Phase = "transferring"
	session.progress.TotalSize = totalSize
	session.progress.TotalFiles = len(files)
	session.mu.Unlock()
	
	// 逐个传输文件
	for _, f := range files {
		if err := p.transferFile(session, f); err != nil {
			p.setSessionError(session, fmt.Sprintf("传输 %s 失败: %v", f.path, err))
			return
		}
	}
	
	// 发送完成消息
	if err := session.conn.WriteJSON(P2PMessage{Type: MsgTypeTransferDone}); err != nil {
		p.setSessionError(session, "发送完成消息失败")
		return
	}
	
	session.mu.Lock()
	session.Status = StatusCompleted
	session.progress.Phase = "completed"
	session.mu.Unlock()
}

type transferFile struct {
	path string
	size int64
}

// transferFile 传输单个文件
func (p *P2PMigrateService) transferFile(session *MigrateSession, tf transferFile) error {
	f, err := os.Open(tf.path)
	if err != nil {
		return err
	}
	defer f.Close()
	
	fileID := generateID()
	totalChunks := int((tf.size + ChunkSize - 1) / ChunkSize)
	if totalChunks == 0 {
		totalChunks = 1
	}
	
	relativePath := strings.TrimPrefix(tf.path, p.dataDir)
	relativePath = strings.TrimPrefix(relativePath, "/")
	
	buf := make([]byte, ChunkSize)
	chunkIdx := 0
	
	for {
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		
		data := buf[:n]
		
		// 计算校验和
		checksum := fmt.Sprintf("%x", sha256.Sum256(data))
		
		// 加密数据
		encData, err := p.encryptData(session.sharedSecret, data)
		if err != nil {
			return err
		}
		
		chunk := ChunkPayload{
			FileID:      fileID,
			FilePath:    relativePath,
			ChunkIdx:    chunkIdx,
			TotalChunks: totalChunks,
			Data:        base64.StdEncoding.EncodeToString(encData),
			Checksum:    checksum,
			IsLast:      chunkIdx == totalChunks-1,
		}
		
		payload, _ := json.Marshal(chunk)
		if err := session.conn.WriteJSON(P2PMessage{
			Type:    MsgTypeChunk,
			Payload: payload,
		}); err != nil {
			return err
		}
		
		// 更新进度
		session.mu.Lock()
		session.progress.TransferredSize += int64(n)
		session.progress.CurrentFile = relativePath
		session.mu.Unlock()
		
		chunkIdx++
		
		if err == io.EOF {
			break
		}
	}
	
	session.mu.Lock()
	session.progress.TransferredFiles++
	session.mu.Unlock()
	
	return nil
}

// encryptData 加密数据
func (p *P2PMigrateService) encryptData(key, plaintext []byte) ([]byte, error) {
	// 使用 SHA256 派生 AES 密钥
	aesKey := sha256.Sum256(key)
	
	block, err := aes.NewCipher(aesKey[:])
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptData 解密数据
func (p *P2PMigrateService) decryptData(key, ciphertext []byte) ([]byte, error) {
	aesKey := sha256.Sum256(key)
	
	block, err := aes.NewCipher(aesKey[:])
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("密文太短")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

// 辅助方法：收集各类文件
func (p *P2PMigrateService) collectConfigFiles() []transferFile {
	var files []transferFile
	configPath := filepath.Join(p.dataDir, "config")
	_ = filepath.Walk(configPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, transferFile{path: path, size: info.Size()})
		return nil
	})
	return files
}

func (p *P2PMigrateService) collectDockerFiles() []transferFile {
	var files []transferFile
	dockerPath := filepath.Join(p.dataDir, "docker")
	_ = filepath.Walk(dockerPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, transferFile{path: path, size: info.Size()})
		return nil
	})
	return files
}

func (p *P2PMigrateService) collectSambaFiles() []transferFile {
	var files []transferFile
	sambaPath := filepath.Join(p.dataDir, "samba")
	_ = filepath.Walk(sambaPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, transferFile{path: path, size: info.Size()})
		return nil
	})
	return files
}

func (p *P2PMigrateService) collectPathFiles(path string) []transferFile {
	var files []transferFile
	_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, transferFile{path: p, size: info.Size()})
		return nil
	})
	return files
}

// applyMigratedContent 应用迁移的内容
func (p *P2PMigrateService) applyMigratedContent(session *MigrateSession) {
	tempDir := filepath.Join(p.dataDir, "migrate-temp")
	
	// 复制配置文件到目标位置
	_ = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		
		relativePath := strings.TrimPrefix(path, tempDir)
		targetPath := filepath.Join(p.dataDir, relativePath)
		
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil
		}
		
		src, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer src.Close()
		
		dst, err := os.Create(targetPath)
		if err != nil {
			return nil
		}
		defer dst.Close()
		
		_, _ = io.Copy(dst, src)
		return nil
	})
	
	// 清理临时目录
	_ = os.RemoveAll(tempDir)
}

// 辅助方法
func (p *P2PMigrateService) setSessionError(session *MigrateSession, errMsg string) {
	session.mu.Lock()
	session.Status = StatusFailed
	if session.progress != nil {
		session.progress.Error = errMsg
	}
	session.mu.Unlock()
}

func (p *P2PMigrateService) heartbeatLoop(session *MigrateSession) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()
	
	for {
		<-ticker.C
		
		session.mu.RLock()
		status := session.Status
		conn := session.conn
		session.mu.RUnlock()
		
		if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
			return
		}
		
		if conn != nil {
			_ = conn.WriteJSON(P2PMessage{Type: MsgTypeHeartbeat})
		}
	}
}

func (p *P2PMigrateService) cleanupExpiredSession(sessionID string, expiry time.Duration) {
	time.Sleep(expiry + time.Minute)
	p.cleanupSession(sessionID)
}

func (p *P2PMigrateService) cleanupSession(sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if session, exists := p.sessions[sessionID]; exists {
		delete(p.pairCodes, session.PairCode)
		delete(p.sessions, sessionID)
	}
}

// generatePairCode 生成配对码 (格式: ABC-123-XYZ)
func generatePairCode() string {
	chars := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 排除容易混淆的字符
	code := make([]byte, 9)
	for i := 0; i < 9; i++ {
		b := make([]byte, 1)
		rand.Read(b)
		code[i] = chars[int(b[0])%len(chars)]
	}
	return fmt.Sprintf("%s-%s-%s", string(code[0:3]), string(code[3:6]), string(code[6:9]))
}

// generateID 生成唯一 ID
func generateID() string {
	return uuid.New().String()
}
