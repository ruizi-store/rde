// Package ai 通用 Webhook 适配器
package ai

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// WebhookAdapter 通用 Webhook 适配器
type WebhookAdapter struct {
	logger  *zap.Logger
	config  *WebhookConfig
	gateway *GatewayService
}

// WebhookRequest Webhook 请求
type WebhookRequest struct {
	UserID         string `json:"user_id" binding:"required"`
	Message        string `json:"message" binding:"required"`
	ConversationID string `json:"conversation_id,omitempty"`
	UserName       string `json:"user_name,omitempty"`
}

// WebhookResponse Webhook 响应
type WebhookResponse struct {
	Success        bool   `json:"success"`
	Response       string `json:"response,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Error          string `json:"error,omitempty"`
}

// NewWebhookAdapter 创建 Webhook 适配器
func NewWebhookAdapter(logger *zap.Logger, config *WebhookConfig, gateway *GatewayService) *WebhookAdapter {
	return &WebhookAdapter{logger: logger, config: config, gateway: gateway}
}

func (w *WebhookAdapter) Platform() PlatformType          { return PlatformWebhook }
func (w *WebhookAdapter) Start(ctx context.Context) error { return nil }
func (w *WebhookAdapter) Stop() error                     { return nil }
func (w *WebhookAdapter) IsEnabled() bool                 { return w.config.Enabled }

func (w *WebhookAdapter) SendMessage(ctx context.Context, msg OutgoingMessage) error {
	return nil // Webhook 是同步的，不主动推送
}

// HandleRequest 处理 HTTP 请求，返回响应文本
func (w *WebhookAdapter) HandleRequest(apiKey string, req WebhookRequest) *WebhookResponse {
	// 验证 API Key
	if apiKey != "Bearer "+w.config.APIKey {
		return &WebhookResponse{Success: false, Error: "Invalid or missing API key"}
	}

	chatID := req.ConversationID
	if chatID == "" {
		chatID = req.UserID
	}

	inMsg := IncomingMessage{
		Platform:  PlatformWebhook,
		UserID:    req.UserID,
		UserName:  req.UserName,
		ChatID:    chatID,
		Text:      req.Message,
		Timestamp: time.Now(),
	}

	response, err := w.gateway.HandleMessage(context.Background(), inMsg)
	if err != nil {
		return &WebhookResponse{Success: false, Error: "Failed to process message: " + err.Error()}
	}

	return &WebhookResponse{
		Success:        true,
		Response:       response.Text,
		ConversationID: chatID,
	}
}

// HandleStreamRequest 处理流式 Webhook 请求，返回 SSE 事件 channel
func (w *WebhookAdapter) HandleStreamRequest(apiKey string, req WebhookRequest) (<-chan string, error) {
	// 验证 API Key
	if apiKey != "Bearer "+w.config.APIKey {
		return nil, fmt.Errorf("invalid or missing API key")
	}

	chatID := req.ConversationID
	if chatID == "" {
		chatID = req.UserID
	}

	inMsg := IncomingMessage{
		Platform:  PlatformWebhook,
		UserID:    req.UserID,
		UserName:  req.UserName,
		ChatID:    chatID,
		Text:      req.Message,
		Timestamp: time.Now(),
	}

	ch := make(chan string, 10)
	go func() {
		defer close(ch)
		response, err := w.gateway.HandleMessage(context.Background(), inMsg)
		if err != nil {
			ch <- "error: " + err.Error()
			return
		}
		ch <- response.Text
	}()

	return ch, nil
}
