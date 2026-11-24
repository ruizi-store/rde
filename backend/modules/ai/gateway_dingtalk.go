// Package ai 钉钉机器人适配器
//
// 接收：Stream 模式（企业自建应用，WebSocket 长连接，无需公网回调）——
// 先 POST /v1.0/gateway/connections/open 拿 endpoint+ticket，再拨 WebSocket，
// 收 CALLBACK 帧（机器人消息，回执需 ack）、SYSTEM 帧（心跳，回 pong）。
// 发送：优先消息携带的 sessionWebhook（会话级回复）；过期则回落群自定义
// 机器人 webhook（可选加签）。审批通知发 actionCard，按钮指向 Web 审批页。
package ai

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// DingtalkAdapter 钉钉适配器
type DingtalkAdapter struct {
	logger  *zap.Logger
	config  *DingtalkConfig
	gateway *GatewayService
	client  *http.Client

	ctx    context.Context
	cancel context.CancelFunc

	// sessionWebhook 缓存（chatID/userID → webhook URL，会话级回复）
	webhookMu sync.RWMutex
	webhooks  map[string]webhookEntry
}

type webhookEntry struct {
	url     string
	expires time.Time
}

// Stream 帧结构
type DingtalkStreamFrame struct {
	SpecVersion string                `json:"specVersion"`
	Type        string                `json:"type"` // SYSTEM / EVENT / CALLBACK
	Headers     DingtalkFrameHeaders  `json:"headers"`
	Data        json.RawMessage       `json:"data"`
}

type DingtalkFrameHeaders struct {
	Topic       string `json:"topic"`
	ContentType string `json:"contentType"`
	MessageID   string `json:"messageId"`
	Time        int64  `json:"time"`
}

// Stream ack 响应帧
type DingtalkStreamAck struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Headers map[string]interface{} `json:"headers"`
	Data    string                 `json:"data"`
}

// connections/open 响应
type DingtalkOpenConnectionResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Endpoint string `json:"endpoint"`
		Ticket   string `json:"ticket"`
	} `json:"result"`
}

// 机器人消息（CALLBACK data）
type DingtalkBotMessage struct {
	ConversationID   string `json:"conversationId"`
	ConversationType string `json:"conversationType"` // 1 单聊 / 2 群聊
	SenderID         string `json:"senderId"`
	SenderStaffID    string `json:"senderStaffId"`
	SenderNick       string `json:"senderNick"`
	Text             struct {
		Content string `json:"content"`
	} `json:"text"`
	SessionWebhook             string `json:"sessionWebhook"`
	SessionWebhookExpiredTime  int64  `json:"sessionWebhookExpiredTime"`
	CreateAt                   int64  `json:"createAt"`
}

// webhook 发送响应
type DingtalkWebhookResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// NewDingtalkAdapter 创建钉钉适配器
func NewDingtalkAdapter(logger *zap.Logger, config *DingtalkConfig, gateway *GatewayService) *DingtalkAdapter {
	return &DingtalkAdapter{
		logger:  logger,
		config:  config,
		gateway: gateway,
		client:  &http.Client{Timeout: 30 * time.Second},
		webhooks: make(map[string]webhookEntry),
	}
}

func (d *DingtalkAdapter) Platform() PlatformType { return PlatformDingtalk }

func (d *DingtalkAdapter) Start(ctx context.Context) error {
	d.ctx, d.cancel = context.WithCancel(ctx)
	if d.config.ClientID != "" && d.config.ClientSecret != "" {
		go d.streamLoop()
		d.logger.Info("Dingtalk adapter started (stream mode)")
	} else {
		d.logger.Info("Dingtalk adapter started (webhook send-only；接收需配置 ClientID/ClientSecret 走 Stream 模式)")
	}
	return nil
}

func (d *DingtalkAdapter) Stop() error {
	if d.cancel != nil {
		d.cancel()
	}
	return nil
}

func (d *DingtalkAdapter) IsEnabled() bool {
	return d.config.Enabled && (d.config.ClientID != "" ||
		(d.config.WebhookURL != "" && d.config.ClientSecret == ""))
}

// SendMessage 发送消息（审批通知升级为 actionCard）
func (d *DingtalkAdapter) SendMessage(ctx context.Context, msg OutgoingMessage) error {
	target, err := d.resolveSendTarget(msg)
	if err != nil {
		return err
	}

	// 审批通知 → actionCard（按钮指向 Web 审批页；未配置公网地址则发 markdown 文本）
	if approvalID, ok := parseApprovalNotification(msg.Text); ok {
		if link := d.gateway.ApprovalWebLink(approvalID); link != "" {
			return d.postWebhook(target, map[string]interface{}{
				"msgtype": "actionCard",
				"actionCard": map[string]interface{}{
					"title": fmt.Sprintf("🔔 审批请求 #%d", approvalID),
					"text":  msg.Text,
					"btnOrientation": "0",
					"btns": []map[string]string{
						{"title": "打开 Web 审批", "actionURL": link},
					},
				},
			})
		}
		return d.postWebhook(target, map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"title": "审批请求", "text": msg.Text},
		})
	}

	return d.postWebhook(target, map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": msg.Text},
	})
}

// resolveSendTarget 解析发送目标：URL 直发 > sessionWebhook 缓存 > 群机器人 webhook
func (d *DingtalkAdapter) resolveSendTarget(msg OutgoingMessage) (string, error) {
	chatID := msg.ChatID
	if strings.HasPrefix(chatID, "http") {
		return chatID, nil
	}

	d.webhookMu.RLock()
	entry, okChat := d.webhooks[chatID]
	if !okChat && msg.UserID != "" {
		entry, okChat = d.webhooks[msg.UserID]
	}
	d.webhookMu.RUnlock()
	if okChat && time.Now().Before(entry.expires) {
		return entry.url, nil
	}

	if d.config.WebhookURL != "" {
		return d.config.WebhookURL, nil
	}
	return "", fmt.Errorf("钉钉无可用的发送目标（会话 webhook 已过期且未配置群机器人 webhook）")
}

// postWebhook 向 webhook URL 发消息（自定义机器人自动加签）
func (d *DingtalkAdapter) postWebhook(target string, payload map[string]interface{}) error {
	u := target
	if d.config.Secret != "" && strings.Contains(u, "dingtalk") {
		ts := time.Now().UnixMilli()
		stringToSign := fmt.Sprintf("%d\n%s", ts, d.config.Secret)
		h := hmac.New(sha256.New, []byte(d.config.Secret))
		h.Write([]byte(stringToSign))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
		sep := "&"
		if !strings.Contains(u, "?") {
			sep = "?"
		}
		u = fmt.Sprintf("%s%stimestamp=%d&sign=%s", u, sep, ts, sign)
	}

	body, _ := json.Marshal(payload)
	resp, err := d.client.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("钉钉发送失败: %w", err)
	}
	defer resp.Body.Close()

	var result DingtalkWebhookResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ErrCode != 0 {
		return fmt.Errorf("钉钉 webhook 错误: %d - %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

// streamLoop 维护 Stream 长连接（断线自动重连）
func (d *DingtalkAdapter) streamLoop() {
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		if err := d.runStreamConnection(); err != nil {
			d.logger.Error("钉钉 Stream 连接断开，5 秒后重连", zap.Error(err))
		}
		select {
		case <-d.ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// runStreamConnection 建立一次 Stream 连接并处理推送
func (d *DingtalkAdapter) runStreamConnection() error {
	endpoint, ticket, err := d.openConnection()
	if err != nil {
		return fmt.Errorf("open connection: %w", err)
	}

	wsURL := endpoint
	if !strings.Contains(wsURL, "ticket=") {
		sep := "?"
		if strings.Contains(wsURL, "?") {
			sep = "&"
		}
		wsURL += sep + "ticket=" + url.QueryEscape(ticket)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(d.ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer conn.Close()
	d.logger.Info("钉钉 Stream 已连接", zap.String("endpoint", endpoint))

	for {
		select {
		case <-d.ctx.Done():
			conn.Close()
			return nil
		default:
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read frame: %w", err)
		}
		var frame DingtalkStreamFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			d.logger.Warn("钉钉 Stream 帧解析失败", zap.Error(err))
			continue
		}
		if err := d.handleStreamFrame(conn, &frame); err != nil {
			d.logger.Error("钉钉 Stream 帧处理失败", zap.Error(err))
		}
	}
}

// handleStreamFrame 处理一帧推送并回执
func (d *DingtalkAdapter) handleStreamFrame(conn *websocket.Conn, frame *DingtalkStreamFrame) error {
	switch frame.Type {
	case "SYSTEM":
		// 心跳/系统消息：回 pong 保活
		return d.writeAck(conn, frame.Headers.MessageID, "pong")
	case "CALLBACK", "EVENT":
		// 先 ack（消息已接收），业务异步处理
		if err := d.writeAck(conn, frame.Headers.MessageID, `{"response":{}}`); err != nil {
			return err
		}
		if frame.Type == "CALLBACK" && strings.Contains(frame.Headers.Topic, "bot") {
			go d.handleBotMessage(frame.Data)
		}
	default:
		d.logger.Debug("钉钉 Stream 未知帧类型", zap.String("type", frame.Type))
	}
	return nil
}

// writeAck 写响应帧
func (d *DingtalkAdapter) writeAck(conn *websocket.Conn, messageID, data string) error {
	ack := DingtalkStreamAck{
		Code:    200,
		Message: "OK",
		Headers: map[string]interface{}{
			"messageId":   messageID,
			"contentType": "application/json",
		},
		Data: data,
	}
	body, _ := json.Marshal(ack)
	return conn.WriteMessage(websocket.TextMessage, body)
}

// handleBotMessage 处理机器人消息
func (d *DingtalkAdapter) handleBotMessage(data json.RawMessage) {
	var msg DingtalkBotMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		d.logger.Error("钉钉消息解析失败", zap.Error(err))
		return
	}
	text := strings.TrimSpace(msg.Text.Content)
	if text == "" {
		return
	}

	// 缓存会话级回复地址
	if msg.SessionWebhook != "" {
		expire := time.Now().Add(110 * time.Minute) // 官方 2 小时，保守 110 分钟
		if msg.SessionWebhookExpiredTime > 0 {
			if t := time.UnixMilli(msg.SessionWebhookExpiredTime); t.Before(expire) && t.After(time.Now()) {
				expire = t
			}
		}
		d.webhookMu.Lock()
		d.webhooks[msg.ConversationID] = webhookEntry{url: msg.SessionWebhook, expires: expire}
		if msg.SenderStaffID != "" {
			d.webhooks[msg.SenderStaffID] = webhookEntry{url: msg.SessionWebhook, expires: expire}
		}
		d.webhookMu.Unlock()
	}

	userID := msg.SenderStaffID
	if userID == "" {
		userID = msg.SenderID
	}

	inMsg := IncomingMessage{
		Platform:       PlatformDingtalk,
		UserID:         userID,
		UserName:       msg.SenderNick,
		ChatID:         msg.ConversationID,
		Text:           text,
		Timestamp:      time.UnixMilli(msg.CreateAt),
		IsGroupMessage: msg.ConversationType == "2",
	}

	response, err := d.gateway.HandleMessage(context.Background(), inMsg)
	if err != nil {
		d.logger.Error("钉钉消息处理失败", zap.Error(err))
		return
	}
	if response != nil {
		if err := d.SendMessage(context.Background(), *response); err != nil {
			d.logger.Error("钉钉回复发送失败", zap.Error(err))
		}
	}
}

// openConnection 注册 Stream 凭证
func (d *DingtalkAdapter) openConnection() (endpoint, ticket string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"clientId":     d.config.ClientID,
		"clientSecret": d.config.ClientSecret,
		"subscriptions": []map[string]string{
			{"type": "CALLBACK", "topic": "/v1.0/im/bot/messages/get"},
		},
		"ua": "rde-dingtalk-stream/1.0",
	})
	resp, err := d.client.Post(d.apiBase()+"/v1.0/gateway/connections/open",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result DingtalkOpenConnectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}
	if result.Result.Endpoint == "" || result.Result.Ticket == "" {
		return "", "", fmt.Errorf("connections/open 失败: %d - %s", result.Code, result.Message)
	}
	return result.Result.Endpoint, result.Result.Ticket, nil
}

// apiBase 钉钉 API 地址（默认 api.dingtalk.com，测试可注入）
func (d *DingtalkAdapter) apiBase() string {
	if d.config.APIBase != "" {
		return strings.TrimSuffix(d.config.APIBase, "/")
	}
	return "https://api.dingtalk.com"
}
