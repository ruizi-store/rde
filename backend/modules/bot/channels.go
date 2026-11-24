// Package bot — IM 渠道绑定（P3：网关消息路由到 Bot）
//
// Bot.Channels 存 JSON 数组：
//
//	[{"platform":"telegram","chat_id":"-100123","user_id":"456"}]
//
// 匹配规则（网关 HandleMessage 调用 ResolveBotByChannel）：
// platform 必须相等，且 chat_id 或 user_id 任一非空命中。
package bot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// BotChannel 渠道绑定项
type BotChannel struct {
	Platform string `json:"platform" binding:"required"` // telegram / wecom / webhook ...
	ChatID   string `json:"chat_id,omitempty"`            // 群/会话 ID
	UserID   string `json:"user_id,omitempty"`            // 用户 ID
}

// validPlatforms 已支持的平台（与 ai 网关适配器对齐）
var validPlatforms = map[string]bool{
	"telegram": true,
	"wecom":    true,
	"webhook":  true,
}

// GetChannels 解析 Bot 的渠道绑定
func (s *Service) GetChannels(botID string) ([]BotChannel, error) {
	b, err := s.Get(context.Background(), botID)
	if err != nil {
		return nil, err
	}
	return parseChannels(b.Channels)
}

// UpdateChannels 校验并更新渠道绑定（全量覆盖）
func (s *Service) UpdateChannels(botID string, channels []BotChannel) (*Bot, error) {
	b, err := s.Get(context.Background(), botID)
	if err != nil {
		return nil, err
	}
	if len(channels) > 20 {
		return nil, errors.New("渠道绑定数量上限 20")
	}
	for i, ch := range channels {
		ch.Platform = strings.TrimSpace(strings.ToLower(ch.Platform))
		ch.ChatID = strings.TrimSpace(ch.ChatID)
		ch.UserID = strings.TrimSpace(ch.UserID)
		if !validPlatforms[ch.Platform] {
			return nil, errors.New("无效平台（可选 telegram/wecom/webhook）")
		}
		if ch.ChatID == "" && ch.UserID == "" {
			return nil, errors.New("chat_id 与 user_id 不能同时为空")
		}
		channels[i] = ch
	}
	data, err := json.Marshal(channels)
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(&Bot{}).Where("id = ?", botID).Update("channels", string(data)).Error; err != nil {
		return nil, err
	}
	b.Channels = string(data)
	return b, nil
}

// ResolveBotByChannel 按渠道信息解析绑定的 Bot（未命中返回 nil）
// 遍历启用状态的 Bot，按 Bot.Channels 匹配；多条命中取 ID 字典序保证稳定。
func (s *Service) ResolveBotByChannel(platform, chatID, userID string) *Bot {
	platform = strings.TrimSpace(strings.ToLower(platform))
	if platform == "" {
		return nil
	}
	var bots []Bot
	if err := s.db.Where("enabled = ?", true).Find(&bots).Error; err != nil {
		return nil
	}
	var hit *Bot
	for i := range bots {
		channels, err := parseChannels(bots[i].Channels)
		if err != nil {
			continue
		}
		for _, ch := range channels {
			if ch.Platform != platform {
				continue
			}
			match := (ch.ChatID != "" && ch.ChatID == chatID) ||
				(ch.UserID != "" && ch.UserID == userID)
			if match {
				if hit == nil || bots[i].ID < hit.ID {
					hit = &bots[i]
				}
				break
			}
		}
	}
	return hit
}

// parseChannels 解析 Channels JSON（空/非法返回空切片）
func parseChannels(raw string) ([]BotChannel, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []BotChannel{}, nil
	}
	var channels []BotChannel
	if err := json.Unmarshal([]byte(raw), &channels); err != nil {
		return nil, err
	}
	return channels, nil
}
