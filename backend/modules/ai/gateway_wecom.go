// Package ai 企业微信适配器
package ai

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WecomAdapter 企业微信适配器
type WecomAdapter struct {
	logger        *zap.Logger
	config        *WecomConfig
	gateway       *GatewayService
	accessToken   string
	tokenExpireAt time.Time
	tokenMu       sync.RWMutex
	aesKey        []byte
	ctx           context.Context
	cancel        context.CancelFunc
}

// WecomMessage 企业微信消息（XML 格式）
type WecomMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        string   `xml:"MsgId"`
	AgentID      int      `xml:"AgentID"`
	// 模板卡片按钮回调（MsgType=event / Event=template_card_event）
	Event    string `xml:"Event,omitempty"`
	TaskId   string `xml:"TaskId,omitempty"`
	EventKey string `xml:"EventKey,omitempty"`
}

// WecomEncryptedMessage 加密的消息
type WecomEncryptedMessage struct {
	XMLName    xml.Name `xml:"xml"`
	ToUserName string   `xml:"ToUserName"`
	Encrypt    string   `xml:"Encrypt"`
	AgentID    int      `xml:"AgentID"`
}

// WecomTokenResponse Access Token 响应
type WecomTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// WecomSendMessageRequest 发送消息请求
type WecomSendMessageRequest struct {
	ToUser       string               `json:"touser"`
	MsgType      string               `json:"msgtype"`
	AgentID      int                  `json:"agentid"`
	Text         *WecomTextContent    `json:"text,omitempty"`
	TemplateCard *WecomTemplateCard   `json:"template_card,omitempty"`
}

// WecomTextContent 文本内容
type WecomTextContent struct {
	Content string `json:"content"`
}

// WecomTemplateCard 模板卡片（button_list，审批按钮交互）
type WecomTemplateCard struct {
	CardType   string                   `json:"card_type"` // button_list
	Source     *WecomCardSource         `json:"source,omitempty"`
	MainTitle  *WecomCardMainTitle      `json:"main_title,omitempty"`
	TaskID     string                   `json:"task_id"`
	SubTitle   string                   `json:"sub_title_text,omitempty"`
	ButtonList []WecomCardButton        `json:"button_list"`
}

type WecomCardSource struct {
	Desc string `json:"desc"`
}

type WecomCardMainTitle struct {
	Title string `json:"title"`
	Desc  string `json:"desc,omitempty"`
}

// WecomCardButton 卡片按钮（style：1 主要按钮，2 警示/红色，3 普通）
type WecomCardButton struct {
	Text string `json:"text"`
	Type int    `json:"type"` // 1 点击跳转/回调
	Key  string `json:"key"`
	Style int   `json:"style,omitempty"`
}

// NewWecomAdapter 创建企业微信适配器
func NewWecomAdapter(logger *zap.Logger, config *WecomConfig, gateway *GatewayService) *WecomAdapter {
	adapter := &WecomAdapter{
		logger: logger, config: config, gateway: gateway,
	}
	if config.EncodingKey != "" {
		if key, err := base64.StdEncoding.DecodeString(config.EncodingKey + "="); err == nil {
			adapter.aesKey = key
		}
	}
	return adapter
}

func (w *WecomAdapter) Platform() PlatformType { return PlatformWecom }

func (w *WecomAdapter) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)
	if err := w.refreshAccessToken(); err != nil {
		w.logger.Error("Failed to get initial access token", zap.Error(err))
	}
	go w.tokenRefreshLoop()
	w.logger.Info("Wecom adapter started")
	return nil
}

func (w *WecomAdapter) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

func (w *WecomAdapter) IsEnabled() bool {
	return w.config.Enabled && w.config.CorpID != "" && w.config.Secret != ""
}

func (w *WecomAdapter) SendMessage(ctx context.Context, msg OutgoingMessage) error {
	token, err := w.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	reqBody := WecomSendMessageRequest{
		ToUser: msg.UserID, MsgType: "text", AgentID: w.config.AgentID,
		Text: &WecomTextContent{Content: msg.Text},
	}

	// 审批通知 → button_list 模板卡片（按钮回调直接决定）
	if approvalID, ok := parseApprovalNotification(msg.Text); ok {
		reqBody = WecomSendMessageRequest{
			ToUser: msg.UserID, MsgType: "template_card", AgentID: w.config.AgentID,
			TemplateCard: w.approvalCard(approvalID, msg.Text),
		}
	}

	body, _ := json.Marshal(reqBody)
	u := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := http.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom error: %d - %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

// approvalCard 审批模板卡片（task_id=审批编号，按钮 key=approve/deny）
func (w *WecomAdapter) approvalCard(approvalID uint, text string) *WecomTemplateCard {
	// 通知正文去掉命令提示（卡片按钮已替代）
	body := strings.TrimSpace(regexp.MustCompile(`\n?\n?同意回复[^\n]*$`).ReplaceAllString(text, ""))
	return &WecomTemplateCard{
		CardType:  "button_list",
		Source:    &WecomCardSource{Desc: "RDE 数字员工"},
		MainTitle: &WecomCardMainTitle{Title: fmt.Sprintf("🔔 审批请求 #%d", approvalID)},
		TaskID:    fmt.Sprintf("approval_%d", approvalID),
		SubTitle:  body,
		ButtonList: []WecomCardButton{
			{Text: "批准", Type: 1, Key: "approve", Style: 1},
			{Text: "拒绝", Type: 1, Key: "deny", Style: 2},
		},
	}
}

// HandleCallback 处理企业微信回调
func (w *WecomAdapter) HandleCallback(msgSignature, timestamp, nonce, echostr string, body []byte) (string, error) {
	// GET 请求 - URL 验证
	if echostr != "" {
		if w.verifySignature(msgSignature, timestamp, nonce, echostr) {
			decrypted, err := w.decryptMessage(echostr)
			if err != nil {
				return "", err
			}
			return decrypted, nil
		}
		return "", fmt.Errorf("signature verification failed")
	}

	// POST 请求 - 消息回调
	var encMsg WecomEncryptedMessage
	if err := xml.Unmarshal(body, &encMsg); err != nil {
		return "", err
	}

	if !w.verifySignature(msgSignature, timestamp, nonce, encMsg.Encrypt) {
		return "", fmt.Errorf("invalid signature")
	}

	decrypted, err := w.decryptMessage(encMsg.Encrypt)
	if err != nil {
		return "", err
	}

	var msg WecomMessage
	if err := xml.Unmarshal([]byte(decrypted), &msg); err != nil {
		return "", err
	}

	// 模板卡片按钮回调（审批决定）
	if msg.MsgType == "event" && msg.Event == "template_card_event" && msg.TaskId != "" {
		return w.handleCardEvent(&msg)
	}

	inMsg := IncomingMessage{
		Platform:  PlatformWecom,
		UserID:    msg.FromUserName,
		ChatID:    msg.FromUserName,
		Text:      msg.Content,
		MessageID: msg.MsgId,
		Timestamp: time.Unix(msg.CreateTime, 0),
	}

	go func() {
		response, err := w.gateway.HandleMessage(context.Background(), inMsg)
		if err != nil {
			w.logger.Error("Failed to handle message", zap.Error(err))
			return
		}
		if response != nil {
			if err := w.SendMessage(context.Background(), *response); err != nil {
				w.logger.Error("Failed to send response", zap.Error(err))
			}
		}
	}()

	return "success", nil
}

// handleCardEvent 处理审批卡片按钮回调：TaskId=approval_<id>，EventKey=approve/deny
func (w *WecomAdapter) handleCardEvent(msg *WecomMessage) (string, error) {
	taskID := strings.TrimPrefix(msg.TaskId, "approval_")
	approvalID, err := strconv.ParseUint(taskID, 10, 64)
	if err != nil {
		w.logger.Warn("企微卡片回调 TaskId 无效", zap.String("task_id", msg.TaskId))
		return "success", nil
	}

	approve := msg.EventKey == "approve"
	reply, err := w.gateway.HandleApprovalButton(uint(approvalID), approve, "wecom:"+msg.FromUserName)
	if err != nil {
		w.logger.Warn("企微卡片审批失败", zap.Error(err), zap.String("task_id", msg.TaskId))
		reply = "❌ " + err.Error()
	}

	// 结果私发给点击者（卡片本身需调用更新接口才能变化，此处以消息形式确认）
	if msg.FromUserName != "" {
		confirmMsg := OutgoingMessage{
			Platform: PlatformWecom,
			UserID:   msg.FromUserName,
			ChatID:   msg.FromUserName,
			Text:     reply,
		}
		if err := w.SendMessage(context.Background(), confirmMsg); err != nil {
			w.logger.Error("企微审批结果发送失败", zap.Error(err))
		}
	}
	return "success", nil
}

func (w *WecomAdapter) getAccessToken() (string, error) {
	w.tokenMu.RLock()
	if w.accessToken != "" && time.Now().Before(w.tokenExpireAt) {
		token := w.accessToken
		w.tokenMu.RUnlock()
		return token, nil
	}
	w.tokenMu.RUnlock()
	return "", w.refreshAccessToken()
}

func (w *WecomAdapter) refreshAccessToken() error {
	u := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		w.config.CorpID, w.config.Secret)

	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result WecomTokenResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom token error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	w.tokenMu.Lock()
	w.accessToken = result.AccessToken
	w.tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	w.tokenMu.Unlock()
	return nil
}

func (w *WecomAdapter) tokenRefreshLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.refreshAccessToken()
		}
	}
}

func (w *WecomAdapter) verifySignature(msgSignature, timestamp, nonce, encrypt string) bool {
	strs := []string{w.config.Token, timestamp, nonce, encrypt}
	sort.Strings(strs)
	h := sha1.New()
	h.Write([]byte(strings.Join(strs, "")))
	return fmt.Sprintf("%x", h.Sum(nil)) == msgSignature
}

func (w *WecomAdapter) decryptMessage(encrypted string) (string, error) {
	if len(w.aesKey) == 0 {
		return "", fmt.Errorf("AES key not configured")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(w.aesKey)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := w.aesKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	// PKCS7 去填充
	decrypted = w.pkcs7Unpad(decrypted)

	if len(decrypted) < 20 {
		return "", fmt.Errorf("decrypted message too short")
	}

	msgLen := binary.BigEndian.Uint32(decrypted[16:20])
	if int(msgLen) > len(decrypted)-20 {
		return "", fmt.Errorf("invalid message length")
	}
	return string(decrypted[20 : 20+msgLen]), nil
}

// encryptMessage 加密消息（用于回复企业微信的加密回复）
func (w *WecomAdapter) encryptMessage(msg string) (string, error) {
	if len(w.aesKey) == 0 {
		return "", fmt.Errorf("AES key not configured")
	}

	// 构造消息：random(16) + msg_len(4) + msg + receiveid(corp_id)
	msgBytes := []byte(msg)
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msgBytes)))

	random := make([]byte, 16)
	for i := range random {
		random[i] = byte(time.Now().UnixNano() % 256)
	}

	plaintext := bytes.Join([][]byte{random, msgLen, msgBytes, []byte(w.config.CorpID)}, nil)

	// PKCS7 填充
	plaintext = w.pkcs7Pad(plaintext, aes.BlockSize)

	// AES-CBC 加密
	block, err := aes.NewCipher(w.aesKey)
	if err != nil {
		return "", err
	}

	iv := w.aesKey[:aes.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)

	ciphertext := make([]byte, len(plaintext))
	mode.CryptBlocks(ciphertext, plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// pkcs7Pad PKCS7 填充
func (w *WecomAdapter) pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// pkcs7Unpad PKCS7 去填充
func (w *WecomAdapter) pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > len(data) {
		return data
	}
	return data[:len(data)-padding]
}

// GetUserInfo 获取用户信息
func (w *WecomAdapter) GetUserInfo(userID string) (map[string]interface{}, error) {
	token, err := w.getAccessToken()
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/get?access_token=%s&userid=%s", token, userID)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}
