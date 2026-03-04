// Package ai Telegram 适配器
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// TelegramAdapter Telegram 适配器
type TelegramAdapter struct {
	logger  *zap.Logger
	config  *TelegramConfig
	gateway *GatewayService
	baseURL string
	client  *http.Client
	offset  int64
	ctx     context.Context
	cancel  context.CancelFunc
}

// TelegramUpdate Telegram Update 对象
type TelegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message,omitempty"`
}

// TelegramMessage Telegram 消息
type TelegramMessage struct {
	MessageID int64            `json:"message_id"`
	From      *TelegramUser    `json:"from,omitempty"`
	Chat      *TelegramChat    `json:"chat"`
	Date      int64            `json:"date"`
	Text      string           `json:"text,omitempty"`
	ReplyTo   *TelegramMessage `json:"reply_to_message,omitempty"`
}

// TelegramUser Telegram 用户
type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// TelegramChat Telegram 聊天
type TelegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
}

// TelegramResponse Telegram API 响应
type TelegramResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}

// TelegramSendMessage 发送消息请求
type TelegramSendMessage struct {
	ChatID           int64  `json:"chat_id"`
	Text             string `json:"text"`
	ParseMode        string `json:"parse_mode,omitempty"`
	ReplyToMessageID int64  `json:"reply_to_message_id,omitempty"`
}

// NewTelegramAdapter 创建 Telegram 适配器
func NewTelegramAdapter(logger *zap.Logger, config *TelegramConfig, gateway *GatewayService) *TelegramAdapter {
	return NewTelegramAdapterWithTimeout(logger, config, gateway, 60)
}

// NewTelegramAdapterWithTimeout 创建带超时的 Telegram 适配器
func NewTelegramAdapterWithTimeout(logger *zap.Logger, config *TelegramConfig, gateway *GatewayService, timeoutSec int) *TelegramAdapter {
	transport := &http.Transport{}

	switch config.ProxyMode {
	case "system":
		proxyURL := getSystemProxyURL(logger)
		if proxyURL != "" {
			if parsedURL, err := url.Parse(proxyURL); err == nil {
				transport.Proxy = http.ProxyURL(parsedURL)
				logger.Info("Telegram using system proxy", zap.String("proxy", proxyURL))
			}
		}
	case "custom":
		if config.ProxyURL != "" {
			if proxyURL, err := url.Parse(config.ProxyURL); err == nil {
				transport.Proxy = http.ProxyURL(proxyURL)
				logger.Info("Telegram using custom proxy", zap.String("proxy", config.ProxyURL))
			}
		}
	}

	return &TelegramAdapter{
		logger:  logger,
		config:  config,
		gateway: gateway,
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", config.BotToken),
		client: &http.Client{
			Timeout:   time.Duration(timeoutSec) * time.Second,
			Transport: transport,
		},
	}
}

func (t *TelegramAdapter) Platform() PlatformType { return PlatformTelegram }

func (t *TelegramAdapter) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	if err := t.validateToken(); err != nil {
		return fmt.Errorf("invalid bot token: %w", err)
	}

	if t.config.UseWebhook {
		if err := t.setWebhook(); err != nil {
			return fmt.Errorf("failed to set webhook: %w", err)
		}
		t.logger.Info("Telegram webhook mode started")
	} else {
		t.deleteWebhook()
		go t.pollLoop()
		t.logger.Info("Telegram long polling mode started")
	}
	return nil
}

func (t *TelegramAdapter) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

func (t *TelegramAdapter) IsEnabled() bool {
	return t.config.Enabled && t.config.BotToken != ""
}

func (t *TelegramAdapter) SendMessage(ctx context.Context, msg OutgoingMessage) error {
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat_id: %w", err)
	}

	sendMsg := TelegramSendMessage{
		ChatID: chatID, Text: msg.Text, ParseMode: "Markdown",
	}
	if msg.ReplyToMsgID != "" {
		if replyID, err := strconv.ParseInt(msg.ReplyToMsgID, 10, 64); err == nil {
			sendMsg.ReplyToMessageID = replyID
		}
	}

	body, _ := json.Marshal(sendMsg)
	resp, err := t.client.Post(t.baseURL+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result TelegramResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if !result.OK {
		// Markdown 解析失败时降级为纯文本
		if result.ErrorCode == 400 {
			sendMsg.ParseMode = ""
			body, _ = json.Marshal(sendMsg)
			resp2, err := t.client.Post(t.baseURL+"/sendMessage", "application/json", bytes.NewReader(body))
			if err == nil {
				defer resp2.Body.Close()
				json.NewDecoder(resp2.Body).Decode(&result)
			}
		}
		if !result.OK {
			return fmt.Errorf("telegram error: %d - %s", result.ErrorCode, result.Description)
		}
	}
	return nil
}

// SendPhoto 发送图片
func (t *TelegramAdapter) SendPhoto(chatID int64, photoURL string, caption string) error {
	url := fmt.Sprintf("%s/sendPhoto?chat_id=%d&photo=%s&caption=%s",
		t.baseURL, chatID, photoURL, caption)
	resp, err := t.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result TelegramResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.OK {
		return fmt.Errorf("send photo failed: %d - %s", result.ErrorCode, result.Description)
	}
	return nil
}

// SendDocument 发送文件
func (t *TelegramAdapter) SendDocument(chatID int64, documentURL string, caption string) error {
	url := fmt.Sprintf("%s/sendDocument?chat_id=%d&document=%s&caption=%s",
		t.baseURL, chatID, documentURL, caption)
	resp, err := t.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result TelegramResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.OK {
		return fmt.Errorf("send document failed: %d - %s", result.ErrorCode, result.Description)
	}
	return nil
}

// HandleWebhook 处理 Webhook 回调
func (t *TelegramAdapter) HandleWebhook(body []byte) {
	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		t.logger.Error("Failed to parse update", zap.Error(err))
		return
	}
	if update.Message != nil {
		go t.handleMessage(update.Message)
	}
}

// TestConnection 测试连接
func (t *TelegramAdapter) TestConnection() error {
	return t.validateToken()
}

func (t *TelegramAdapter) validateToken() error {
	resp, err := t.client.Get(t.baseURL + "/getMe")
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram API: %w (check proxy settings)", err)
	}
	defer resp.Body.Close()

	var result TelegramResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.OK {
		return fmt.Errorf("invalid token: %s", result.Description)
	}

	var bot TelegramUser
	json.Unmarshal(result.Result, &bot)
	t.logger.Info("Telegram bot validated", zap.String("username", bot.Username))
	return nil
}

func (t *TelegramAdapter) setWebhook() error {
	u := fmt.Sprintf("%s/setWebhook?url=%s", t.baseURL, t.config.WebhookURL)
	resp, err := t.client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result TelegramResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.OK {
		return fmt.Errorf("set webhook failed: %s", result.Description)
	}
	return nil
}

func (t *TelegramAdapter) deleteWebhook() {
	resp, err := t.client.Get(t.baseURL + "/deleteWebhook")
	if err == nil {
		resp.Body.Close()
	}
}

func (t *TelegramAdapter) pollLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			updates, err := t.getUpdates()
			if err != nil {
				t.logger.Error("Failed to get updates", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			for _, update := range updates {
				t.offset = update.UpdateID + 1
				if update.Message != nil {
					go t.handleMessage(update.Message)
				}
			}
		}
	}
}

func (t *TelegramAdapter) getUpdates() ([]TelegramUpdate, error) {
	u := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=30", t.baseURL, t.offset)
	resp, err := t.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result TelegramResponse
	json.Unmarshal(body, &result)
	if !result.OK {
		return nil, fmt.Errorf("get updates failed: %s", result.Description)
	}

	var updates []TelegramUpdate
	json.Unmarshal(result.Result, &updates)
	return updates, nil
}

func (t *TelegramAdapter) handleMessage(msg *TelegramMessage) {
	if msg.Text == "" {
		return
	}

	userName := msg.From.FirstName
	if msg.From.LastName != "" {
		userName += " " + msg.From.LastName
	}

	inMsg := IncomingMessage{
		Platform:       PlatformTelegram,
		UserID:         strconv.FormatInt(msg.From.ID, 10),
		UserName:       userName,
		ChatID:         strconv.FormatInt(msg.Chat.ID, 10),
		Text:           msg.Text,
		MessageID:      strconv.FormatInt(msg.MessageID, 10),
		Timestamp:      time.Unix(msg.Date, 0),
		IsGroupMessage: msg.Chat.Type != "private",
	}
	if msg.ReplyTo != nil {
		inMsg.ReplyToMsgID = strconv.FormatInt(msg.ReplyTo.MessageID, 10)
	}

	response, err := t.gateway.HandleMessage(context.Background(), inMsg)
	if err != nil {
		t.logger.Error("Failed to handle message", zap.Error(err))
		return
	}
	if response != nil {
		if err := t.SendMessage(context.Background(), *response); err != nil {
			t.logger.Error("Failed to send response", zap.Error(err))
		}
	}
}

// getSystemProxyURL 从系统配置获取代理地址
func getSystemProxyURL(logger *zap.Logger) string {
	proxyFile := "/etc/profile.d/rde-proxy.sh"
	if proxyURL := parseProxyFromFile(proxyFile); proxyURL != "" {
		return proxyURL
	}
	for _, env := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if proxyURL := os.Getenv(env); proxyURL != "" {
			return proxyURL
		}
	}
	return ""
}

func parseProxyFromFile(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	var httpsProxy, httpProxy string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "export https_proxy=") || strings.HasPrefix(line, "export HTTPS_PROXY=") {
			httpsProxy = extractProxyURL(line)
		} else if strings.HasPrefix(line, "export http_proxy=") || strings.HasPrefix(line, "export HTTP_PROXY=") {
			if httpProxy == "" {
				httpProxy = extractProxyURL(line)
			}
		}
	}
	if httpsProxy != "" {
		return httpsProxy
	}
	return httpProxy
}

func extractProxyURL(line string) string {
	idx := strings.Index(line, "=")
	if idx == -1 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(line[idx+1:]), "\"'")
}
