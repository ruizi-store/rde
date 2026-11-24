// Package ai 飞书（自建应用机器人）适配器
//
// 发送：open.feishu.cn 开放 API（tenant_access_token，消息/卡片）。
// 接收：事件订阅回调 POST /api/v1/ai/gateway/feishu/callback
// （url_verification 握手、im.message.receive_v1 消息、card.action.trigger 审批按钮）。
// 审批通知发送 interactive 卡片，按钮回调直接决定审批（无需会话）。
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FeishuAdapter 飞书适配器
type FeishuAdapter struct {
	logger        *zap.Logger
	config        *FeishuConfig
	gateway       *GatewayService
	client        *http.Client
	tenantToken   string
	tokenExpireAt time.Time
	tokenMu       sync.RWMutex
}

// 飞书 API 响应通用结构
type FeishuAPIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data,omitempty"`
}

// tenant_access_token 响应
type FeishuTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

// 事件订阅请求（v2 schema）
type FeishuEventRequest struct {
	Schema  string          `json:"schema"`
	Type    string          `json:"type"` // url_verification / event_callback
	Event   json.RawMessage `json:"event,omitempty"`
	Challenge string        `json:"challenge,omitempty"`
	Header  *FeishuEventHeader `json:"header,omitempty"`
}

type FeishuEventHeader struct {
	EventID   string `json:"event_id"`
 EventType string `json:"event_type"`
	Token     string `json:"token"`
}

// im.message.receive_v1 事件
type FeishuMessageEvent struct {
	Sender struct {
		SenderID struct {
			OpenID string `json:"open_id"`
			UserID string `json:"user_id"`
		} `json:"sender_id"`
		SenderType string `json:"sender_type"`
	} `json:"sender"`
	Message struct {
		MessageID   string `json:"message_id"`
		ChatID      string `json:"chat_id"`
		MessageType string `json:"message_type"`
		Content     string `json:"content"` // JSON 字符串
		CreateTime  string `json:"create_time"`
	} `json:"message"`
}

// card.action.trigger 事件（卡片按钮回调）
type FeishuCardActionEvent struct {
	Operator struct {
		OperatorID struct {
			OpenID string `json:"open_id"`
			UserID string `json:"user_id"`
		} `json:"operator_id"`
	} `json:"operator"`
	Action struct {
		Value map[string]interface{} `json:"value"`
	} `json:"action"`
}

// 飞书消息内容（text）
type FeishuTextContent struct {
	Text string `json:"text"`
}

// NewFeishuAdapter 创建飞书适配器
func NewFeishuAdapter(logger *zap.Logger, config *FeishuConfig, gateway *GatewayService) *FeishuAdapter {
	return &FeishuAdapter{
		logger:  logger,
		config:  config,
		gateway: gateway,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *FeishuAdapter) Platform() PlatformType { return PlatformFeishu }

func (f *FeishuAdapter) Start(ctx context.Context) error {
	f.logger.Info("Feishu adapter started",
		zap.Bool("app_configured", f.IsEnabled()),
		zap.String("callback", "/api/v1/ai/gateway/feishu/callback"))
	return nil
}

func (f *FeishuAdapter) Stop() error { return nil }

func (f *FeishuAdapter) IsEnabled() bool {
	return f.config.Enabled && f.config.AppID != "" && f.config.AppSecret != ""
}

// SendMessage 发送消息（审批通知自动升级为 interactive 卡片）
func (f *FeishuAdapter) SendMessage(ctx context.Context, msg OutgoingMessage) error {
	if !f.IsEnabled() {
		return fmt.Errorf("飞书未配置应用凭证")
	}
	if msg.ChatID == "" {
		return fmt.Errorf("飞书消息缺少 chat_id（Bot 渠道绑定需填群 chat_id）")
	}

	// 审批通知 → 交互式卡片（按钮回调直接决定）
	if approvalID, ok := parseApprovalNotification(msg.Text); ok {
		return f.sendApprovalCard(msg.ChatID, approvalID, msg.Text)
	}

	content, _ := json.Marshal(FeishuTextContent{Text: msg.Text})
	return f.sendIMMessage(msg.ChatID, "text", string(content))
}

// sendApprovalCard 发送审批卡片（按钮回调 + Web 链接兜底）
func (f *FeishuAdapter) sendApprovalCard(chatID string, approvalID uint, text string) error {
	// 通知正文去掉命令提示（卡片按钮已替代）
	body := strings.TrimSpace(regexp.MustCompile(`\n?\n?同意回复[^\n]*$`).ReplaceAllString(text, ""))
	elements := []map[string]interface{}{
		{"tag": "div", "text": map[string]interface{}{"tag": "lark_md", "content": body}},
		{"tag": "hr"},
		{"tag": "action", "actions": []map[string]interface{}{
			{"tag": "button", "text": map[string]interface{}{"tag": "plain_text", "content": "✅ 批准"},
				"type": "primary", "value": map[string]interface{}{"approval_id": approvalID, "approve": true}},
			{"tag": "button", "text": map[string]interface{}{"tag": "plain_text", "content": "🚫 拒绝"},
				"type": "danger", "value": map[string]interface{}{"approval_id": approvalID, "approve": false}},
		}},
	}
	// Web 链接兜底（无回调能力时仍可一键到 Web 决定）
	if link := f.gateway.ApprovalWebLink(approvalID); link != "" {
		elements = append(elements, map[string]interface{}{
			"tag": "note", "elements": []map[string]interface{}{
				{"tag": "plain_text", "content": "或打开 Web 审批：" + link},
			},
		})
	} else {
		elements = append(elements, map[string]interface{}{
			"tag": "note", "elements": []map[string]interface{}{
				{"tag": "plain_text", "content": fmt.Sprintf("回复 /approve %d 或 /deny %d", approvalID, approvalID)},
			},
		})
	}

	card := map[string]interface{}{
		"config": map[string]interface{}{"wide_screen_mode": true},
		"header": map[string]interface{}{
			"title":   map[string]interface{}{"tag": "plain_text", "content": fmt.Sprintf("🔔 审批请求 #%d", approvalID)},
			"template": "orange",
		},
		"elements": elements,
	}
	cardJSON, _ := json.Marshal(card)
	return f.sendIMMessage(chatID, "interactive", string(cardJSON))
}

// sendIMMessage 调用 im/v1/messages 发送
func (f *FeishuAdapter) sendIMMessage(chatID, msgType, content string) error {
	token, err := f.getTenantToken()
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{
		"receive_id": chatID,
		"msg_type":   msgType,
		"content":    content,
	})
	u := f.apiBase() + "/open-apis/im/v1/messages?receive_id_type=chat_id"
	req, _ := http.NewRequest("POST", u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("飞书发送失败: %w", err)
	}
	defer resp.Body.Close()

	var result FeishuAPIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Code != 0 {
		return fmt.Errorf("飞书 API 错误: %d - %s", result.Code, result.Msg)
	}
	return nil
}

// getTenantToken 获取（缓存）tenant_access_token
func (f *FeishuAdapter) getTenantToken() (string, error) {
	f.tokenMu.RLock()
	if f.tenantToken != "" && time.Now().Before(f.tokenExpireAt) {
		token := f.tenantToken
		f.tokenMu.RUnlock()
		return token, nil
	}
	f.tokenMu.RUnlock()

	body, _ := json.Marshal(map[string]string{
		"app_id":     f.config.AppID,
		"app_secret": f.config.AppSecret,
	})
	resp, err := f.client.Post(f.apiBase()+"/open-apis/auth/v3/tenant_access_token/internal",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("获取飞书 token 失败: %w", err)
	}
	defer resp.Body.Close()

	var result FeishuTokenResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Code != 0 || result.TenantAccessToken == "" {
		return "", fmt.Errorf("飞书 token 错误: %d - %s", result.Code, result.Msg)
	}

	f.tokenMu.Lock()
	f.tenantToken = result.TenantAccessToken
	f.tokenExpireAt = time.Now().Add(time.Duration(result.Expire-300) * time.Second)
	f.tokenMu.Unlock()
	return result.TenantAccessToken, nil
}

// HandleCallback 处理事件订阅回调，返回 HTTP 响应体
func (f *FeishuAdapter) HandleCallback(body []byte) map[string]interface{} {
	var req FeishuEventRequest
	if err := json.Unmarshal(body, &req); err != nil {
		f.logger.Error("飞书回调解析失败", zap.Error(err))
		return map[string]interface{}{"code": 400, "msg": "bad request"}
	}

	// URL 验证握手
	if req.Type == "url_verification" && req.Challenge != "" {
		return map[string]interface{}{"challenge": req.Challenge}
	}

	// 事件（v2 schema：header 里带 event_type/token）
	eventType := ""
	token := ""
	if req.Header != nil {
		eventType = req.Header.EventType
		token = req.Header.Token
	} else if req.Schema == "2.0" && len(req.Event) > 0 {
		// 兼容 header 缺失的宽松解析
		var h struct {
			Header struct {
				EventType string `json:"event_type"`
				Token     string `json:"token"`
			} `json:"header"`
		}
		_ = json.Unmarshal(body, &h)
		eventType = h.Header.EventType
		token = h.Header.Token
	}

	// 校验 VerificationToken（配置了才校验，防伪造事件）
	if f.config.VerificationToken != "" && token != "" && token != f.config.VerificationToken {
		f.logger.Warn("飞书回调 token 校验失败")
		return map[string]interface{}{"code": 403, "msg": "invalid token"}
	}

	switch eventType {
	case "im.message.receive_v1":
		var event FeishuMessageEvent
		if err := json.Unmarshal(req.Event, &event); err != nil {
			f.logger.Error("飞书消息事件解析失败", zap.Error(err))
			break
		}
		go f.handleMessageEvent(&event)
	case "card.action.trigger":
		var event FeishuCardActionEvent
		if err := json.Unmarshal(req.Event, &event); err != nil {
			f.logger.Error("飞书卡片回调解析失败", zap.Error(err))
			break
		}
		go f.handleCardAction(&event)
	default:
		f.logger.Debug("飞书事件忽略", zap.String("event_type", eventType))
	}

	return map[string]interface{}{"code": 0}
}

// handleMessageEvent 处理收到的消息
func (f *FeishuAdapter) handleMessageEvent(event *FeishuMessageEvent) {
	if event.Message.MessageType != "text" {
		return
	}
	var content FeishuTextContent
	if err := json.Unmarshal([]byte(event.Message.Content), &content); err != nil {
		f.logger.Error("飞书消息内容解析失败", zap.Error(err))
		return
	}
	// 群聊 @机器人 会带 @_user_N 前缀，剥离
	text := stripFeishuMentions(content.Text)
	if strings.TrimSpace(text) == "" {
		return
	}

	userID := event.Sender.SenderID.OpenID
	if userID == "" {
		userID = event.Sender.SenderID.UserID
	}

	inMsg := IncomingMessage{
		Platform: PlatformFeishu,
		UserID:   userID,
		ChatID:   event.Message.ChatID,
		Text:     text,
		MessageID: event.Message.MessageID,
		Timestamp: time.Now(),
	}

	response, err := f.gateway.HandleMessage(context.Background(), inMsg)
	if err != nil {
		f.logger.Error("飞书消息处理失败", zap.Error(err))
		return
	}
	if response != nil {
		if err := f.SendMessage(context.Background(), *response); err != nil {
			f.logger.Error("飞书回复发送失败", zap.Error(err))
		}
	}
}

// handleCardAction 处理审批卡片按钮回调
func (f *FeishuAdapter) handleCardAction(event *FeishuCardActionEvent) {
	approvalID := 0
	approve := false
	if v, ok := event.Action.Value["approval_id"].(float64); ok {
		approvalID = int(v)
	}
	if v, ok := event.Action.Value["approve"].(bool); ok {
		approve = v
	}
	if approvalID <= 0 {
		return
	}

	byUser := event.Operator.OperatorID.OpenID
	if byUser == "" {
		byUser = event.Operator.OperatorID.UserID
	}
	reply, err := f.gateway.HandleApprovalButton(uint(approvalID), approve, "feishu:"+byUser)
	if err != nil {
		f.logger.Warn("飞书卡片审批失败", zap.Error(err), zap.Int("approval_id", approvalID))
		reply = "❌ " + err.Error()
	}
	f.logger.Info("飞书卡片审批完成", zap.Int("approval_id", approvalID), zap.String("result", reply))
}

// apiBase API 地址（默认 open.feishu.cn，测试可注入）
func (f *FeishuAdapter) apiBase() string {
	if f.config.APIBase != "" {
		return strings.TrimSuffix(f.config.APIBase, "/")
	}
	return "https://open.feishu.cn"
}

// stripFeishuMentions 剥离 @_user_N 形式的 @ 提及
func stripFeishuMentions(text string) string {
	re := regexp.MustCompile(`@_user_\d+\s*`)
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}
