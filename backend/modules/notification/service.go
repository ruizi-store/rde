// Package notification 通知服务
package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service 通知服务
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
	hub    *WebSocketHub
}

// NewService 创建通知服务
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		db:     db,
		logger: logger,
		hub:    NewWebSocketHub(),
	}
}

// SetHub 设置 WebSocket Hub
func (s *Service) SetHub(hub *WebSocketHub) {
	s.hub = hub
}

// GetHub 获取 WebSocket Hub
func (s *Service) GetHub() *WebSocketHub {
	return s.hub
}

// Migrate 迁移数据库
func (s *Service) Migrate() error {
	return s.db.AutoMigrate(
		&Notification{},
		&NotificationSettings{},
		&NotificationChannel{},
		&NotificationRule{},
		&NotificationHistory{},
	)
}

// ==================== 站内通知 ====================

// ListNotifications 获取通知列表
func (s *Service) ListNotifications(ctx context.Context, userID string, req *ListNotificationsRequest) (*NotificationListResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	query := s.db.Model(&Notification{}).Where("user_id = ? OR user_id = ''", userID)

	// 类别筛选
	if req.Category != "" {
		categories := strings.Split(req.Category, ",")
		query = query.Where("category IN ?", categories)
	}

	// 级别筛选
	if req.Severity != "" {
		severities := strings.Split(req.Severity, ",")
		query = query.Where("severity IN ?", severities)
	}

	// 已读筛选
	if req.IsRead != nil {
		query = query.Where("is_read = ?", *req.IsRead)
	}

	// 时间筛选
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	// 总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	var items []*Notification
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}

	// 未读数量
	var unreadCount int64
	s.db.Model(&Notification{}).Where("(user_id = ? OR user_id = '') AND is_read = ?", userID, false).Count(&unreadCount)

	return &NotificationListResponse{
		Items:       items,
		Total:       total,
		Page:        req.Page,
		PageSize:    req.PageSize,
		UnreadCount: unreadCount,
	}, nil
}

// GetUnreadCount 获取未读数量
func (s *Service) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := s.db.Model(&Notification{}).Where("(user_id = ? OR user_id = '') AND is_read = ?", userID, false).Count(&count).Error
	return count, err
}

// MarkAsRead 标记为已读
func (s *Service) MarkAsRead(ctx context.Context, userID, id string) error {
	now := time.Now()
	result := s.db.Model(&Notification{}).
		Where("id = ? AND (user_id = ? OR user_id = '')", id, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": &now,
		})
	if result.RowsAffected == 0 {
		return fmt.Errorf("通知不存在")
	}
	return result.Error
}

// MarkAllAsRead 全部标记已读
func (s *Service) MarkAllAsRead(ctx context.Context, userID string) error {
	now := time.Now()
	return s.db.Model(&Notification{}).
		Where("(user_id = ? OR user_id = '') AND is_read = ?", userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": &now,
		}).Error
}

// DeleteNotification 删除通知
func (s *Service) DeleteNotification(ctx context.Context, userID, id string) error {
	result := s.db.Where("id = ? AND (user_id = ? OR user_id = '')", id, userID).Delete(&Notification{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("通知不存在")
	}
	return result.Error
}

// DeleteReadNotifications 删除已读通知
func (s *Service) DeleteReadNotifications(ctx context.Context, userID string) error {
	return s.db.Where("(user_id = ? OR user_id = '') AND is_read = ?", userID, true).Delete(&Notification{}).Error
}

// DeleteAllNotifications 删除所有通知
func (s *Service) DeleteAllNotifications(ctx context.Context, userID string) error {
	return s.db.Where("user_id = ? OR user_id = ''", userID).Delete(&Notification{}).Error
}

// SendNotification 发送站内通知
func (s *Service) SendNotification(ctx context.Context, req *SendNotificationRequest) (*Notification, error) {
	notification := &Notification{
		ID:        uuid.New().String(),
		UserID:    req.UserID,
		Category:  req.Category,
		Severity:  req.Severity,
		Title:     req.Title,
		Content:   req.Content,
		Link:      req.Link,
		Icon:      s.getCategoryIcon(req.Category),
		Source:    req.Source,
		IsRead:    false,
		CreatedAt: time.Now(),
	}

	if err := s.db.Create(notification).Error; err != nil {
		return nil, err
	}

	s.logger.Info("Created notification",
		zap.String("id", notification.ID),
		zap.String("category", string(notification.Category)),
		zap.String("title", notification.Title))

	// 通过 WebSocket 推送
	if s.hub != nil {
		s.hub.BroadcastNotification(notification)
	}

	// 触发外发规则
	go s.triggerPushRules(ctx, notification)

	return notification, nil
}

// getCategoryIcon 获取类别图标
func (s *Service) getCategoryIcon(category Category) string {
	icons := map[Category]string{
		CategorySystem:   "computer",
		CategorySecurity: "shield",
		CategoryStorage:  "hard-drive",
		CategoryBackup:   "archive",
		CategoryApp:      "package",
		CategoryUpdate:   "refresh-cw",
	}
	if icon, ok := icons[category]; ok {
		return icon
	}
	return "bell"
}

// ==================== 通知设置 ====================

// GetSettings 获取通知设置
func (s *Service) GetSettings(ctx context.Context, userID string) (*SettingsResponse, error) {
	var settings NotificationSettings
	err := s.db.Where("user_id = ?", userID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		// 返回默认设置
		return s.getDefaultSettings(), nil
	}
	if err != nil {
		return nil, err
	}

	var categories []string
	var severities []string
	if settings.FilterCategories != "" {
		json.Unmarshal([]byte(settings.FilterCategories), &categories)
	}
	if settings.FilterSeverities != "" {
		json.Unmarshal([]byte(settings.FilterSeverities), &severities)
	}

	// 如果为空，使用默认值
	if len(categories) == 0 {
		for _, c := range AllCategories {
			categories = append(categories, string(c))
		}
	}
	if len(severities) == 0 {
		for _, s := range AllSeverities {
			severities = append(severities, string(s))
		}
	}

	return &SettingsResponse{
		Enabled:          settings.Enabled,
		DesktopNotify:    settings.DesktopNotify,
		SoundEnabled:     settings.SoundEnabled,
		DndEnabled:       settings.DndEnabled,
		DndFrom:          settings.DndFrom,
		DndTo:            settings.DndTo,
		FilterCategories: categories,
		FilterSeverities: severities,
	}, nil
}

// getDefaultSettings 获取默认设置
func (s *Service) getDefaultSettings() *SettingsResponse {
	categories := make([]string, len(AllCategories))
	for i, c := range AllCategories {
		categories[i] = string(c)
	}
	severities := make([]string, len(AllSeverities))
	for i, sv := range AllSeverities {
		severities[i] = string(sv)
	}
	return &SettingsResponse{
		Enabled:          true,
		DesktopNotify:    true,
		SoundEnabled:     true,
		DndEnabled:       false,
		DndFrom:          "22:00",
		DndTo:            "08:00",
		FilterCategories: categories,
		FilterSeverities: severities,
	}
}

// UpdateSettings 更新通知设置
func (s *Service) UpdateSettings(ctx context.Context, userID string, req *UpdateSettingsRequest) (*SettingsResponse, error) {
	var settings NotificationSettings
	err := s.db.Where("user_id = ?", userID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		settings = NotificationSettings{
			ID:            uuid.New().String(),
			UserID:        userID,
			Enabled:       true,
			DesktopNotify: true,
			SoundEnabled:  true,
			DndEnabled:    false,
			DndFrom:       "22:00",
			DndTo:         "08:00",
			CreatedAt:     time.Now(),
		}
	} else if err != nil {
		return nil, err
	}

	// 更新字段
	if req.Enabled != nil {
		settings.Enabled = *req.Enabled
	}
	if req.DesktopNotify != nil {
		settings.DesktopNotify = *req.DesktopNotify
	}
	if req.SoundEnabled != nil {
		settings.SoundEnabled = *req.SoundEnabled
	}
	if req.DndEnabled != nil {
		settings.DndEnabled = *req.DndEnabled
	}
	if req.DndFrom != "" {
		settings.DndFrom = req.DndFrom
	}
	if req.DndTo != "" {
		settings.DndTo = req.DndTo
	}
	if req.FilterCategories != nil {
		data, _ := json.Marshal(req.FilterCategories)
		settings.FilterCategories = string(data)
	}
	if req.FilterSeverities != nil {
		data, _ := json.Marshal(req.FilterSeverities)
		settings.FilterSeverities = string(data)
	}
	settings.UpdatedAt = time.Now()

	if err := s.db.Save(&settings).Error; err != nil {
		return nil, err
	}

	return s.GetSettings(ctx, userID)
}

// ==================== 推送渠道 ====================

// CreateChannel 创建渠道
func (s *Service) CreateChannel(ctx context.Context, userID string, req *CreateChannelRequest) (*NotificationChannel, error) {
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("配置序列化失败: %w", err)
	}

	channel := &NotificationChannel{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        req.Name,
		Type:        req.Type,
		Config:      string(configJSON),
		Description: req.Description,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(channel).Error; err != nil {
		return nil, err
	}

	s.logger.Info("Created notification channel",
		zap.String("id", channel.ID),
		zap.String("type", string(channel.Type)))

	return channel, nil
}

// GetChannel 获取渠道
func (s *Service) GetChannel(ctx context.Context, id string) (*NotificationChannel, error) {
	var channel NotificationChannel
	if err := s.db.First(&channel, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// ListChannels 列出渠道
func (s *Service) ListChannels(ctx context.Context, userID string) ([]*NotificationChannel, error) {
	var channels []*NotificationChannel
	if err := s.db.Where("user_id = ? OR user_id = ''", userID).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// UpdateChannel 更新渠道
func (s *Service) UpdateChannel(ctx context.Context, id string, req *UpdateChannelRequest) (*NotificationChannel, error) {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		channel.Name = req.Name
	}
	if req.Description != "" {
		channel.Description = req.Description
	}
	if req.Config != nil {
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			return nil, fmt.Errorf("配置序列化失败: %w", err)
		}
		channel.Config = string(configJSON)
	}
	if req.Enabled != nil {
		channel.Enabled = *req.Enabled
	}
	channel.UpdatedAt = time.Now()

	if err := s.db.Save(channel).Error; err != nil {
		return nil, err
	}

	return channel, nil
}

// DeleteChannel 删除渠道
func (s *Service) DeleteChannel(ctx context.Context, id string) error {
	// 删除相关规则
	s.db.Where("channel_id = ?", id).Delete(&NotificationRule{})
	return s.db.Delete(&NotificationChannel{}, "id = ?", id).Error
}

// TestChannel 测试渠道
func (s *Service) TestChannel(ctx context.Context, id string) error {
	channel, err := s.GetChannel(ctx, id)
	if err != nil {
		return err
	}

	return s.sendToChannel(channel, "测试通知", "这是一条来自 RDE 的测试通知，如果您收到此消息，说明通知渠道配置正确。")
}

// ==================== 推送规则 ====================

// CreateRule 创建规则
func (s *Service) CreateRule(ctx context.Context, userID string, req *CreateRuleRequest) (*NotificationRule, error) {
	// 验证渠道存在
	if _, err := s.GetChannel(ctx, req.ChannelID); err != nil {
		return nil, fmt.Errorf("通知渠道不存在")
	}

	categoriesJSON, _ := json.Marshal(req.Categories)
	severitiesJSON, _ := json.Marshal(req.Severities)

	rule := &NotificationRule{
		ID:         uuid.New().String(),
		UserID:     userID,
		Name:       req.Name,
		ChannelID:  req.ChannelID,
		Categories: string(categoriesJSON),
		Severities: string(severitiesJSON),
		Enabled:    true,
		Cooldown:   req.Cooldown,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.db.Create(rule).Error; err != nil {
		return nil, err
	}

	return rule, nil
}

// GetRule 获取规则
func (s *Service) GetRule(ctx context.Context, id string) (*NotificationRule, error) {
	var rule NotificationRule
	if err := s.db.First(&rule, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListRules 列出规则
func (s *Service) ListRules(ctx context.Context, userID string) ([]*NotificationRule, error) {
	var rules []*NotificationRule
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// UpdateRule 更新规则
func (s *Service) UpdateRule(ctx context.Context, id string, req *UpdateRuleRequest) (*NotificationRule, error) {
	rule, err := s.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Categories != nil {
		data, _ := json.Marshal(req.Categories)
		rule.Categories = string(data)
	}
	if req.Severities != nil {
		data, _ := json.Marshal(req.Severities)
		rule.Severities = string(data)
	}
	if req.Cooldown != nil {
		rule.Cooldown = *req.Cooldown
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	rule.UpdatedAt = time.Now()

	if err := s.db.Save(rule).Error; err != nil {
		return nil, err
	}

	return rule, nil
}

// DeleteRule 删除规则
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	return s.db.Delete(&NotificationRule{}, "id = ?", id).Error
}

// ==================== 推送历史 ====================

// GetHistory 获取推送历史
func (s *Service) GetHistory(ctx context.Context, page, pageSize int) ([]*NotificationHistory, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	var total int64
	if err := s.db.Model(&NotificationHistory{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var history []*NotificationHistory
	offset := (page - 1) * pageSize
	if err := s.db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&history).Error; err != nil {
		return nil, 0, err
	}

	return history, total, nil
}

// ==================== 推送逻辑 ====================

// triggerPushRules 触发推送规则
func (s *Service) triggerPushRules(ctx context.Context, notification *Notification) {
	var rules []*NotificationRule
	if err := s.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		s.logger.Error("Failed to query push rules", zap.Error(err))
		return
	}

	for _, rule := range rules {
		if s.shouldTriggerRule(rule, notification) {
			s.executeRule(ctx, rule, notification)
		}
	}
}

// shouldTriggerRule 判断是否触发规则
func (s *Service) shouldTriggerRule(rule *NotificationRule, notification *Notification) bool {
	// 检查冷却时间
	if rule.Cooldown > 0 && rule.LastSentAt != nil {
		if time.Since(*rule.LastSentAt) < time.Duration(rule.Cooldown)*time.Second {
			return false
		}
	}

	// 检查类别
	if rule.Categories != "" {
		var categories []string
		json.Unmarshal([]byte(rule.Categories), &categories)
		if len(categories) > 0 {
			found := false
			for _, c := range categories {
				if c == string(notification.Category) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// 检查级别
	if rule.Severities != "" {
		var severities []string
		json.Unmarshal([]byte(rule.Severities), &severities)
		if len(severities) > 0 {
			found := false
			for _, sv := range severities {
				if sv == string(notification.Severity) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// executeRule 执行推送规则
func (s *Service) executeRule(ctx context.Context, rule *NotificationRule, notification *Notification) {
	channel, err := s.GetChannel(ctx, rule.ChannelID)
	if err != nil {
		s.logger.Error("Failed to get channel", zap.String("channel_id", rule.ChannelID), zap.Error(err))
		return
	}

	if !channel.Enabled {
		return
	}

	// 发送通知
	err = s.sendToChannel(channel, notification.Title, notification.Content)

	// 记录历史
	history := &NotificationHistory{
		ID:          uuid.New().String(),
		ChannelID:   channel.ID,
		ChannelType: channel.Type,
		Category:    notification.Category,
		Title:       notification.Title,
		Content:     notification.Content,
		CreatedAt:   time.Now(),
	}

	if err != nil {
		history.Status = "failed"
		history.ErrorMsg = err.Error()
		s.logger.Error("Failed to send notification", zap.Error(err))
	} else {
		history.Status = "sent"
		now := time.Now()
		history.SentAt = &now
	}

	s.db.Create(history)

	// 更新规则的 LastSentAt
	if err == nil {
		now := time.Now()
		s.db.Model(rule).Update("last_sent_at", &now)
	}
}

// sendToChannel 发送到渠道
func (s *Service) sendToChannel(channel *NotificationChannel, title, content string) error {
	switch channel.Type {
	case ChannelEmail:
		return s.sendEmail(channel, title, content)
	case ChannelTelegram:
		return s.sendTelegram(channel, title, content)
	case ChannelBark:
		return s.sendBark(channel, title, content)
	case ChannelWeChat:
		return s.sendWeChat(channel, title, content)
	case ChannelDingTalk:
		return s.sendDingTalk(channel, title, content)
	case ChannelWebhook:
		return s.sendWebhook(channel, title, content)
	default:
		return fmt.Errorf("不支持的渠道类型: %s", channel.Type)
	}
}

// sendEmail 发送邮件
func (s *Service) sendEmail(channel *NotificationChannel, title, content string) error {
	var config EmailConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("邮件配置解析失败: %w", err)
	}

	if config.SMTPHost == "" || len(config.ToAddresses) == 0 {
		return fmt.Errorf("邮件配置不完整")
	}

	from := config.FromAddress
	if config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from,
		strings.Join(config.ToAddresses, ","),
		title,
		content,
	)

	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)
	var auth smtp.Auth
	if config.SMTPUsername != "" {
		auth = smtp.PlainAuth("", config.SMTPUsername, config.SMTPPassword, config.SMTPHost)
	}

	if config.UseTLS {
		return s.sendEmailTLS(addr, auth, config.FromAddress, config.ToAddresses, []byte(msg), config)
	}

	return smtp.SendMail(addr, auth, config.FromAddress, config.ToAddresses, []byte(msg))
}

// sendEmailTLS 通过 TLS 发送邮件
func (s *Service) sendEmailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, config EmailConfig) error {
	tlsConfig := &tls.Config{
		ServerName: config.SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	if err = client.Mail(from); err != nil {
		return err
	}

	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// sendTelegram 发送 Telegram 消息
func (s *Service) sendTelegram(channel *NotificationChannel, title, content string) error {
	var config TelegramConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("Telegram 配置解析失败: %w", err)
	}

	if config.BotToken == "" || config.ChatID == "" {
		return fmt.Errorf("Telegram 配置不完整")
	}

	text := fmt.Sprintf("*%s*\n\n%s", title, content)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.BotToken)

	payload := map[string]interface{}{
		"chat_id":    config.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	return s.httpPost(apiURL, payload)
}

// sendBark 发送 Bark 推送
func (s *Service) sendBark(channel *NotificationChannel, title, content string) error {
	var config BarkConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("Bark 配置解析失败: %w", err)
	}

	if config.ServerURL == "" || config.DeviceKey == "" {
		return fmt.Errorf("Bark 配置不完整")
	}

	serverURL := strings.TrimSuffix(config.ServerURL, "/")
	apiURL := fmt.Sprintf("%s/%s/%s/%s", serverURL, config.DeviceKey, url.PathEscape(title), url.PathEscape(content))

	resp, err := http.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Bark 推送失败: %s", string(body))
	}

	return nil
}

// sendWeChat 发送企业微信消息
func (s *Service) sendWeChat(channel *NotificationChannel, title, content string) error {
	var config WeChatConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("企业微信配置解析失败: %w", err)
	}

	if config.WebhookURL == "" {
		return fmt.Errorf("企业微信配置不完整")
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("## %s\n\n%s", title, content),
		},
	}

	return s.httpPost(config.WebhookURL, payload)
}

// sendDingTalk 发送钉钉消息
func (s *Service) sendDingTalk(channel *NotificationChannel, title, content string) error {
	var config DingTalkConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("钉钉配置解析失败: %w", err)
	}

	if config.WebhookURL == "" {
		return fmt.Errorf("钉钉配置不完整")
	}

	webhookURL := config.WebhookURL
	if config.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := s.calcDingTalkSign(timestamp, config.Secret)
		webhookURL = fmt.Sprintf("%s&timestamp=%d&sign=%s", config.WebhookURL, timestamp, sign)
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  fmt.Sprintf("## %s\n\n%s", title, content),
		},
	}

	return s.httpPostURL(webhookURL, payload)
}

// calcDingTalkSign 计算钉钉签名
func (s *Service) calcDingTalkSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
}

// sendWebhook 发送 Webhook
func (s *Service) sendWebhook(channel *NotificationChannel, title, content string) error {
	var config WebhookConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("Webhook 配置解析失败: %w", err)
	}

	if config.URL == "" {
		return fmt.Errorf("Webhook 配置不完整")
	}

	payload := map[string]interface{}{
		"title":   title,
		"content": content,
	}

	return s.httpPostURL(config.URL, payload)
}

// httpPost HTTP POST 请求
func (s *Service) httpPost(url string, payload interface{}) error {
	return s.httpPostURL(url, payload)
}

// httpPostURL HTTP POST 请求
func (s *Service) httpPostURL(url string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("请求失败 (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}
