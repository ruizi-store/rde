package bot

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupApprovals(t *testing.T) (*ApprovalService, *Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Bot{}, &BotApproval{}))
	svc := NewService(db, zap.NewNop(), t.TempDir(), &fakeDocker{running: true})
	return NewApprovalService(db, svc, zap.NewNop()), svc
}

func TestApprovalService_Approve(t *testing.T) {
	a, svc := setupApprovals(t)
	b, err := svc.Create("审批员", "", "", "", "")
	require.NoError(t, err)

	// 异步等待审批（带通知回调验证）
	approvedCh := make(chan bool, 1)
	notifyTexts := []string{}
	a.SetNotifier(func(ctx context.Context, platform, chatID, userID, text string) error {
		notifyTexts = append(notifyTexts, text)
		return nil
	})
	_, err = svc.UpdateChannels(b.ID, []BotChannel{{Platform: "webhook", ChatID: "room-1"}})
	require.NoError(t, err)

	go func() {
		approved, msg, err := a.RequestApproval(context.Background(), b.ID, "删除 /data 目录", "清理过期数据", 10)
		require.NoError(t, err)
		approvedCh <- approved
		_ = msg
	}()

	// 等审批落库
	var approval BotApproval
	require.Eventually(t, func() bool {
		return a.db.Where("bot_id = ?", b.ID).First(&approval).Error == nil
	}, 5*time.Second, 100*time.Millisecond)
	assert.Equal(t, ApprovalPending, approval.Status)

	// 通知已推送（含命令提示与操作描述）
	require.Len(t, notifyTexts, 1)
	assert.Contains(t, notifyTexts[0], "/approve")
	assert.Contains(t, notifyTexts[0], "删除 /data 目录")

	// 决定：批准
	reply, err := a.DecideApproval(b.ID, approval.ID, true, "tester")
	require.NoError(t, err)
	assert.Contains(t, reply, "已批准")

	select {
	case ok := <-approvedCh:
		assert.True(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("等待者未被唤醒")
	}

	// 状态落库
	got, err := a.Get(b.ID, approval.ID)
	require.NoError(t, err)
	assert.Equal(t, ApprovalApproved, got.Status)
	assert.Equal(t, "tester", got.DecidedBy)
}

func TestApprovalService_Deny(t *testing.T) {
	a, svc := setupApprovals(t)
	b, err := svc.Create("审批员2", "", "", "", "")
	require.NoError(t, err)

	approval := &BotApproval{BotID: b.ID, Action: "对外发邮件", Reason: "日报推送",
		Status: ApprovalPending, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, a.db.Create(approval).Error)

	reply, err := a.DecideApproval(b.ID, approval.ID, false, "boss")
	require.NoError(t, err)
	assert.Contains(t, reply, "已拒绝")

	got, _ := a.Get(b.ID, approval.ID)
	assert.Equal(t, ApprovalDenied, got.Status)

	// 重复决定被拒绝
	_, err = a.DecideApproval(b.ID, approval.ID, true, "x")
	assert.ErrorContains(t, err, "已处理")
}

func TestApprovalService_Timeout(t *testing.T) {
	a, svc := setupApprovals(t)
	b, err := svc.Create("等待员", "", "", "", "")
	require.NoError(t, err)

	start := time.Now()
	approved, msg, err := a.RequestApproval(context.Background(), b.ID, "危险操作", "", 2)
	require.NoError(t, err)
	assert.False(t, approved)
	assert.Contains(t, msg, "超时")
	assert.Less(t, time.Since(start), 10*time.Second)

	// 超时后状态为 expired
	var got BotApproval
	require.NoError(t, a.db.Where("bot_id = ?", b.ID).First(&got).Error)
	assert.Equal(t, ApprovalExpired, got.Status)
}

func TestApprovalService_OwnerCheck(t *testing.T) {
	a, svc := setupApprovals(t)
	b1, _ := svc.Create("甲", "", "", "", "")
	b2, _ := svc.Create("乙", "", "", "", "")

	approval := &BotApproval{BotID: b1.ID, Action: "x", Status: ApprovalPending, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, a.db.Create(approval).Error)

	// 其他 Bot 不能决定
	_, err := a.DecideApproval(b2.ID, approval.ID, true, "u")
	assert.ErrorContains(t, err, "不属于")

	// List 过滤
	require.NoError(t, a.db.Create(&BotApproval{BotID: b2.ID, Action: "y",
		Status: ApprovalApproved, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	pending, err := a.List(b1.ID, ApprovalPending, 10)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, int64(1), a.PendingCount(b1.ID))
}

func TestApprovalService_WebUIDecidePolling(t *testing.T) {
	a, svc := setupApprovals(t)
	b, _ := svc.Create("轮询员", "", "", "", "")

	resultCh := make(chan bool, 1)
	go func() {
		approved, _, err := a.RequestApproval(context.Background(), b.ID, "操作", "", 10)
		require.NoError(t, err)
		resultCh <- approved
	}()

	// 等审批出现
	var approval BotApproval
	require.Eventually(t, func() bool {
		return a.db.Where("bot_id = ?", b.ID).First(&approval).Error == nil
	}, 5*time.Second, 100*time.Millisecond)

	// 模拟 Web UI 决定：绕过 waiter 通道直接改库（兜底轮询应能感知）
	require.NoError(t, a.db.Model(&BotApproval{}).Where("id = ?", approval.ID).
		Update("status", ApprovalApproved).Error)

	select {
	case ok := <-resultCh:
		assert.True(t, ok)
	case <-time.After(10 * time.Second):
		t.Fatal("兜底轮询未感知决定")
	}
}
