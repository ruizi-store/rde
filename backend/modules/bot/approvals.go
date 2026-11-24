// Package bot — 高风险操作人工审批（P4）
//
// LLM 通过 request_approval 工具发起审批：请求写入 bots_approvals 表并推送
// 到 Bot 绑定渠道（IM 命令 /approve|/deny <id>）与 Web UI，工具阻塞轮询
// 直到决定或超时，把结果返回给 LLM 继续执行。
package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 审批状态
const (
	ApprovalPending = "pending"
	ApprovalApproved = "approved"
	ApprovalDenied   = "denied"
	ApprovalExpired  = "expired"
)

// BotApproval 审批记录
type BotApproval struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	BotID       string     `json:"bot_id" gorm:"index;size:36;not null"`
	Action      string     `json:"action" gorm:"type:text;not null"`
	Reason      string     `json:"reason" gorm:"type:text"`
	Status      string     `json:"status" gorm:"size:16;default:'pending';index"`
	RequestedBy string     `json:"requested_by" gorm:"size:128"` // 发起方（LLM 会话用户）
	DecidedBy   string     `json:"decided_by" gorm:"size:128"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TableName 指定表名
func (BotApproval) TableName() string { return "bots_approvals" }

// ApprovalService 审批管理（实现 ai.ApprovalManager）
type ApprovalService struct {
	db     *gorm.DB
	bots   *Service
	logger *zap.Logger

	// 通知推送（经 ai.Module.SendToChannel）
	notify func(ctx context.Context, platform, chatID, userID, text string) error

	// 决定事件的等待者（approval ID → chan）
	mu      sync.Mutex
	waiters map[uint]chan string
}

// NewApprovalService 创建审批服务
func NewApprovalService(db *gorm.DB, bots *Service, logger *zap.Logger) *ApprovalService {
	return &ApprovalService{
		db:      db,
		bots:    bots,
		logger:  logger,
		waiters: make(map[uint]chan string),
	}
}

// SetNotifier 注入渠道推送函数（bot.Module Init 调用）
func (a *ApprovalService) SetNotifier(notify func(ctx context.Context, platform, chatID, userID, text string) error) {
	a.notify = notify
}

// RequestApproval 发起审批并等待（实现 ai.ApprovalManager）
func (a *ApprovalService) RequestApproval(ctx context.Context, botID, action, reason string, timeoutSec int) (bool, string, error) {
	if timeoutSec <= 0 || timeoutSec > 300 {
		timeoutSec = 120
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return false, "", fmt.Errorf("审批操作不能为空")
	}

	approval := &BotApproval{
		BotID:     botID,
		Action:    action,
		Reason:    reason,
		Status:    ApprovalPending,
		ExpiresAt: time.Now().Add(time.Duration(timeoutSec) * time.Second).Add(5 * time.Minute),
	}
	if err := a.db.Create(approval).Error; err != nil {
		return false, "", err
	}

	// 推送通知到 Bot 绑定渠道
	a.pushNotification(ctx, approval)

	// 阻塞轮询等待决定
	notifyCh := make(chan string, 1)
	a.mu.Lock()
	a.waiters[approval.ID] = notifyCh
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.waiters, approval.ID)
		a.mu.Unlock()
	}()

	timer := time.NewTimer(time.Duration(timeoutSec) * time.Second)
	defer timer.Stop()

	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.expire(approval.ID)
			return false, "调用方取消等待", nil
		case status := <-notifyCh:
			return a.resultMessage(approval.ID, status)
		case <-timer.C:
			a.expire(approval.ID)
			return false, fmt.Sprintf("等待超时（%ds），操作未获批准，请勿执行", timeoutSec), nil
		case <-pollTicker.C:
			// 兜底轮询（Web UI 决定不经 waiter 通道时仍能感知）
			var current BotApproval
			if err := a.db.First(&current, approval.ID).Error; err == nil && current.Status != ApprovalPending {
				return a.resultMessage(approval.ID, current.Status)
			}
		}
	}
}

// DecideApproval 用户决定（实现 ai.ApprovalManager；IM 命令与 Web UI 共用）
func (a *ApprovalService) DecideApproval(botID string, approvalID uint, approve bool, byUser string) (string, error) {
	var approval BotApproval
	if err := a.db.Where("id = ? AND bot_id = ?", approvalID, botID).First(&approval).Error; err != nil {
		return "", fmt.Errorf("审批 #%d 不存在或不属于该 Bot", approvalID)
	}
	if approval.Status != ApprovalPending {
		return "", fmt.Errorf("审批 #%d 已处理（%s）", approvalID, statusText(approval.Status))
	}

	status := ApprovalDenied
	if approve {
		status = ApprovalApproved
	}
	now := time.Now()
	if err := a.db.Model(&approval).Updates(map[string]interface{}{
		"status":     status,
		"decided_by": byUser,
		"decided_at": now,
	}).Error; err != nil {
		return "", err
	}

	// 唤醒等待中的工具调用
	a.mu.Lock()
	if ch, ok := a.waiters[approvalID]; ok {
		select {
		case ch <- status:
		default:
		}
	}
	a.mu.Unlock()

	if approve {
		return fmt.Sprintf("✅ 审批 #%d 已批准（%s），Bot 将继续执行", approvalID, approval.Action), nil
	}
	return fmt.Sprintf("🚫 审批 #%d 已拒绝（%s）", approvalID, approval.Action), nil
}

// List 审批列表（status 为空返回全部）
func (a *ApprovalService) List(botID, status string, limit int) ([]BotApproval, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := a.db.Where("bot_id = ?", botID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var approvals []BotApproval
	err := q.Order("id DESC").Limit(limit).Find(&approvals).Error
	return approvals, err
}

// Get 获取单条
func (a *ApprovalService) Get(botID string, approvalID uint) (*BotApproval, error) {
	var approval BotApproval
	if err := a.db.Where("id = ? AND bot_id = ?", approvalID, botID).First(&approval).Error; err != nil {
		return nil, err
	}
	return &approval, nil
}

// PendingCount 待审数量
func (a *ApprovalService) PendingCount(botID string) int64 {
	var count int64
	a.db.Model(&BotApproval{}).Where("bot_id = ? AND status = ?", botID, ApprovalPending).Count(&count)
	return count
}

// pushNotification 推送审批请求到 Bot 绑定渠道
func (a *ApprovalService) pushNotification(ctx context.Context, approval *BotApproval) {
	if a.notify == nil {
		return
	}
	b, err := a.bots.Get(ctx, approval.BotID)
	if err != nil {
		return
	}
	channels, err := parseChannels(b.Channels)
	if err != nil || len(channels) == 0 {
		return
	}
	text := fmt.Sprintf("🔔 审批请求 #%d（%s）\n操作：%s\n原因：%s\n\n同意回复 /approve %d，拒绝回复 /deny %d",
		approval.ID, b.Name, approval.Action, approval.Reason, approval.ID, approval.ID)
	for _, ch := range channels {
		if err := a.notify(ctx, ch.Platform, ch.ChatID, ch.UserID, text); err != nil {
			a.logger.Warn("approval notify failed", zap.String("platform", ch.Platform), zap.Error(err))
		}
	}
}

// expire 把超时审批置为 expired
func (a *ApprovalService) expire(approvalID uint) {
	a.db.Model(&BotApproval{}).Where("id = ? AND status = ?", approvalID, ApprovalPending).
		Update("status", ApprovalExpired)
}

// resultMessage 按状态返回给 LLM 的说明
func (a *ApprovalService) resultMessage(approvalID uint, status string) (bool, string, error) {
	switch status {
	case ApprovalApproved:
		return true, "已获批准，可以执行", nil
	case ApprovalDenied:
		return false, "用户拒绝了该操作，不要执行", nil
	default:
		return false, fmt.Sprintf("审批状态：%s，操作未获批准，请勿执行", statusText(status)), nil
	}
}

// statusText 状态文案
func statusText(status string) string {
	switch status {
	case ApprovalPending:
		return "待审批"
	case ApprovalApproved:
		return "已批准"
	case ApprovalDenied:
		return "已拒绝"
	case ApprovalExpired:
		return "已超时"
	default:
		return status
	}
}
