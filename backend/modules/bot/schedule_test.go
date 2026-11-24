package bot

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/ruizi-store/rde/backend/modules/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupSchedule(t *testing.T) (*ScheduleService, *Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Bot{}, &BotSchedule{}))
	svc := NewService(db, zap.NewNop(), t.TempDir(), &fakeDocker{running: true})
	return NewScheduleService(db, zap.NewNop(), svc), svc, db
}

func TestScheduleService_CRUD(t *testing.T) {
	s, svc, _ := setupSchedule(t)
	b, err := svc.Create("调度员", "定时汇报", "", "", "")
	require.NoError(t, err)

	// 创建（校验 cron）
	sch := &BotSchedule{Name: "日报", CronExpr: "0 9 * * *", Prompt: "生成今日日报", Enabled: true}
	require.NoError(t, s.Create(b.ID, sch))
	assert.NotZero(t, sch.ID)
	assert.Equal(t, 1, s.ScheduledCount())

	// 无效 cron 拒绝
	err = s.Create(b.ID, &BotSchedule{Name: "x", CronExpr: "bad", Prompt: "p"})
	assert.ErrorContains(t, err, "无效 cron 表达式")

	// 空提示词拒绝
	err = s.Create(b.ID, &BotSchedule{Name: "x", CronExpr: "@every 1h", Prompt: " "})
	assert.ErrorContains(t, err, "提示词")

	// 列表
	list, err := s.List(b.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// 更新（改 cron + 禁用 → 摘除调度）
	updated, err := s.Update(b.ID, sch.ID, map[string]interface{}{"cron": "@every 30m", "enabled": false})
	require.NoError(t, err)
	assert.Equal(t, "@every 30m", updated.CronExpr)
	assert.False(t, updated.Enabled)
	assert.Equal(t, 0, s.ScheduledCount())

	// 更新非法 cron 拒绝
	_, err = s.Update(b.ID, sch.ID, map[string]interface{}{"cron": "not-a-cron"})
	assert.ErrorContains(t, err, "无效 cron 表达式")

	// 删除
	require.NoError(t, s.Delete(b.ID, sch.ID))
	list, _ = s.List(b.ID)
	assert.Empty(t, list)
}

func TestScheduleService_RunNow(t *testing.T) {
	s, svc, _ := setupSchedule(t)
	b, err := svc.Create("跑任务", "职责", "", "", "")
	require.NoError(t, err)
	_, err = svc.UpdateChannels(b.ID, []BotChannel{{Platform: "webhook", ChatID: "room-1"}})
	require.NoError(t, err)

	var mu sync.Mutex
	runPrompt := ""
	sentMsgs := []string{}
	s.SetAIRunner(
		func(ctx context.Context, bot *ai.BotInfo, prompt string) (string, error) {
			mu.Lock()
			runPrompt = prompt
			mu.Unlock()
			assert.Equal(t, b.ID, bot.ID)
			return "TASK_RESULT_42", nil
		},
		func(ctx context.Context, platform, chatID, userID, text string) error {
			mu.Lock()
			sentMsgs = append(sentMsgs, platform+"/"+chatID+":"+text)
			mu.Unlock()
			return nil
		},
	)

	sch := &BotSchedule{Name: "即时", CronExpr: "0 9 * * *", Prompt: "汇报当前进度", Enabled: true}
	require.NoError(t, s.Create(b.ID, sch))

	result, err := s.RunNow(b.ID, sch.ID)
	require.NoError(t, err)
	assert.Equal(t, "TASK_RESULT_42", result)
	assert.Equal(t, "汇报当前进度", runPrompt)

	// 执行结果落库
	list, _ := s.List(b.ID)
	require.Len(t, list, 1)
	assert.Equal(t, "TASK_RESULT_42", list[0].LastResult)
	assert.NotNil(t, list[0].LastRunAt)

	// 推送到绑定渠道
	assert.Equal(t, []string{"webhook/room-1:TASK_RESULT_42"}, sentMsgs)
}

func TestScheduleService_ExecuteError(t *testing.T) {
	s, svc, _ := setupSchedule(t)
	b, err := svc.Create("失败员", "", "", "", "")
	require.NoError(t, err)

	s.SetAIRunner(
		func(ctx context.Context, bot *ai.BotInfo, prompt string) (string, error) {
			return "", assert.AnError
		},
		nil,
	)

	sch := &BotSchedule{Name: "会失败", CronExpr: "@every 1h", Prompt: "p", Enabled: true}
	require.NoError(t, s.Create(b.ID, sch))

	_, err = s.RunNow(b.ID, sch.ID)
	require.Error(t, err)

	list, _ := s.List(b.ID)
	require.Len(t, list, 1)
	assert.NotEmpty(t, list[0].LastError)
	assert.Empty(t, list[0].LastResult)
}

func TestScheduleService_CronFires(t *testing.T) {
	s, svc, _ := setupSchedule(t)
	b, err := svc.Create("触发员", "", "", "", "")
	require.NoError(t, err)

	fired := make(chan string, 1)
	s.SetAIRunner(
		func(ctx context.Context, bot *ai.BotInfo, prompt string) (string, error) {
			fired <- prompt
			return "done", nil
		},
		nil,
	)

	// @every 1s（robfig 秒级描述符）
	sch := &BotSchedule{Name: "每秒", CronExpr: "@every 1s", Prompt: "tick", Enabled: true}
	require.NoError(t, s.Create(b.ID, sch))
	require.NoError(t, s.Start())

	select {
	case p := <-fired:
		assert.Equal(t, "tick", p)
	case <-time.After(5 * time.Second):
		t.Fatal("cron 未触发")
	}
	s.Stop()
}

func TestScheduleService_StartNoRunner(t *testing.T) {
	s, _, _ := setupSchedule(t)
	// 未注入 runner 时 Start 不报错（降级）
	require.NoError(t, s.Start())
	s.Stop()
}
