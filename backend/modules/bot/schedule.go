// Package bot — 定时任务调度（P4：Bot 主动汇报）
//
// 每条日程 = cron 表达式 + 任务提示词；触发时以该 Bot 的身份跑带工具的
// 对话循环（沙箱/技能/记忆可用），结果推送到 Bot 绑定的 IM 渠道。
// cron 语法：robfig/cron 标准格式（5 字段 分 时 日 月 周）+ @every 1h30m 等描述符。
package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/ruizi-store/rde/backend/modules/ai"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BotSchedule 定时任务
type BotSchedule struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	BotID      string     `json:"bot_id" gorm:"index;size:36;not null"`
	Name       string     `json:"name" gorm:"size:128;not null"`
	CronExpr   string     `json:"cron" gorm:"size:64;not null"`
	Prompt     string     `json:"prompt" gorm:"type:text;not null"`
	Enabled    bool       `json:"enabled" gorm:"default:true"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastResult string     `json:"last_result" gorm:"type:text"`
	LastError  string     `json:"last_error" gorm:"size:512"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (BotSchedule) TableName() string { return "bots_schedules" }

// ScheduleService 定时调度服务
type ScheduleService struct {
	db     *gorm.DB
	logger *zap.Logger
	// aiRun 以 Bot 身份执行任务（ai.Module 实现）
	aiRun func(ctx context.Context, bot *ai.BotInfo, prompt string) (string, error)
	// aiSend 推送文本到渠道（ai.Module 实现）
	aiSend func(ctx context.Context, platform, chatID, userID, text string) error
	// bots 提供 Bot 查询与渠道解析
	bots *Service

	cron    *cron.Cron
	mu      sync.Mutex
	entries map[uint]cron.EntryID // schedule ID → cron entry
}

// NewScheduleService 创建调度服务
func NewScheduleService(db *gorm.DB, logger *zap.Logger, bots *Service) *ScheduleService {
	return &ScheduleService{
		db:     db,
		logger: logger,
		bots:   bots,
		cron:   cron.New(),
		entries: make(map[uint]cron.EntryID),
	}
}

// SetAIRunner 注入 ai 模块的任务执行与渠道推送能力（bot.Module Init 调用）
func (s *ScheduleService) SetAIRunner(
	run func(ctx context.Context, bot *ai.BotInfo, prompt string) (string, error),
	send func(ctx context.Context, platform, chatID, userID, text string) error,
) {
	s.aiRun = run
	s.aiSend = send
}

// Start 启动调度器（加载所有启用的日程）
func (s *ScheduleService) Start() error {
	if s.aiRun == nil {
		s.logger.Warn("schedule service: ai runner not injected, schedules disabled")
		return nil
	}
	var schedules []BotSchedule
	if err := s.db.Where("enabled = ?", true).Find(&schedules).Error; err != nil {
		return err
	}
	for i := range schedules {
		if err := s.addEntry(&schedules[i]); err != nil {
			s.logger.Warn("failed to schedule task",
				zap.String("name", schedules[i].Name), zap.Error(err))
		}
	}
	s.cron.Start()
	s.logger.Info("schedule service started", zap.Int("tasks", len(schedules)))
	return nil
}

// Stop 停止调度器
func (s *ScheduleService) Stop() {
	ctx := s.cron.Stop() // 等待运行中的任务结束
	<-ctx.Done()
}

// List 列出 Bot 的日程
func (s *ScheduleService) List(botID string) ([]BotSchedule, error) {
	var schedules []BotSchedule
	err := s.db.Where("bot_id = ?", botID).Order("id").Find(&schedules).Error
	return schedules, err
}

// Create 创建日程并挂到调度器
func (s *ScheduleService) Create(botID string, sch *BotSchedule) error {
	if strings.TrimSpace(sch.Name) == "" {
		return fmt.Errorf("日程名称不能为空")
	}
	if strings.TrimSpace(sch.Prompt) == "" {
		return fmt.Errorf("任务提示词不能为空")
	}
	sch.BotID = botID
	sch.CronExpr = strings.TrimSpace(sch.CronExpr)
	if _, err := cron.ParseStandard(sch.CronExpr); err != nil {
		return fmt.Errorf("无效 cron 表达式 '%s'（5 字段：分 时 日 月 周，或 @every 30m）", sch.CronExpr)
	}
	if err := s.db.Create(sch).Error; err != nil {
		return err
	}
	if sch.Enabled {
		return s.addEntry(sch)
	}
	return nil
}

// Update 更新日程（重新挂载）；updates 的键为 JSON 字段名（name/cron/prompt/enabled）
func (s *ScheduleService) Update(botID string, id uint, updates map[string]interface{}) (*BotSchedule, error) {
	var sch BotSchedule
	if err := s.db.Where("bot_id = ? AND id = ?", botID, id).First(&sch).Error; err != nil {
		return nil, err
	}

	// 白名单映射：JSON 字段名 → 列名
	columns := map[string]interface{}{}
	for key, value := range updates {
		switch key {
		case "name":
			if name, ok := value.(string); ok && name != "" {
				columns["name"] = name
			}
		case "cron":
			expr, ok := value.(string)
			if !ok {
				continue
			}
			expr = strings.TrimSpace(expr)
			if _, err := cron.ParseStandard(expr); err != nil {
				return nil, fmt.Errorf("无效 cron 表达式 '%s'", expr)
			}
			columns["cron_expr"] = expr
		case "prompt":
			if prompt, ok := value.(string); ok && strings.TrimSpace(prompt) != "" {
				columns["prompt"] = prompt
			}
		case "enabled":
			if enabled, ok := value.(bool); ok {
				columns["enabled"] = enabled
			}
		}
	}
	if len(columns) > 0 {
		if err := s.db.Model(&sch).Updates(columns).Error; err != nil {
			return nil, err
		}
	}
	if err := s.db.First(&sch, id).Error; err != nil {
		return nil, err
	}

	// 重新挂载
	s.removeEntry(sch.ID)
	if sch.Enabled {
		if err := s.addEntry(&sch); err != nil {
			return nil, err
		}
	}
	return &sch, nil
}

// Delete 删除日程
func (s *ScheduleService) Delete(botID string, id uint) error {
	if err := s.db.Where("bot_id = ? AND id = ?", botID, id).Delete(&BotSchedule{}).Error; err != nil {
		return err
	}
	s.removeEntry(id)
	return nil
}

// RunNow 立即执行一次（不等 cron 触发；测试/手动验证用）
func (s *ScheduleService) RunNow(botID string, id uint) (string, error) {
	var sch BotSchedule
	if err := s.db.Where("bot_id = ? AND id = ?", botID, id).First(&sch).Error; err != nil {
		return "", err
	}
	return s.execute(context.Background(), &sch)
}

// addEntry 把日程挂到 cron 调度器
func (s *ScheduleService) addEntry(sch *BotSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[sch.ID]; exists {
		return nil
	}
	entryID, err := s.cron.AddFunc(sch.CronExpr, func() {
		var fresh BotSchedule
		if err := s.db.First(&fresh, sch.ID).Error; err != nil {
			return
		}
		if !fresh.Enabled {
			return
		}
		if _, err := s.execute(context.Background(), &fresh); err != nil {
			s.logger.Warn("scheduled task failed",
				zap.String("name", fresh.Name), zap.Error(err))
		}
	})
	if err != nil {
		return err
	}
	s.entries[sch.ID] = entryID
	return nil
}

// removeEntry 摘除调度
func (s *ScheduleService) removeEntry(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}
}

// ScheduledCount 当前挂载的日程数（测试/状态用）
func (s *ScheduleService) ScheduledCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// execute 执行一条日程：以 Bot 身份跑任务 → 推送绑定渠道
func (s *ScheduleService) execute(ctx context.Context, sch *BotSchedule) (string, error) {
	if s.aiRun == nil {
		return "", fmt.Errorf("ai runner 未注入")
	}

	b, err := s.bots.Get(ctx, sch.BotID)
	if err != nil {
		return "", err
	}
	botInfo := &ai.BotInfo{
		ID:          b.ID,
		Name:        b.Name,
		Description: b.Description,
		Model:       b.Model,
		ProviderID:  b.ProviderID,
	}

	result, err := s.aiRun(ctx, botInfo, sch.Prompt)
	now := time.Now()

	record := map[string]interface{}{
		"last_run_at": now,
		"last_result": truncate(result, 4096),
		"last_error":  "",
	}
	if err != nil {
		record["last_result"] = ""
		record["last_error"] = truncate(err.Error(), 512)
		s.db.Model(&BotSchedule{}).Where("id = ?", sch.ID).Updates(record)
		return "", err
	}
	s.db.Model(&BotSchedule{}).Where("id = ?", sch.ID).Updates(record)

	// 推送到 Bot 绑定的所有渠道（失败只告警不回滚执行结果）
	if s.aiSend != nil {
		channels, _ := parseChannels(b.Channels)
		pushed := 0
		for _, ch := range channels {
			if err := s.aiSend(ctx, ch.Platform, ch.ChatID, ch.UserID, result); err != nil {
				s.logger.Warn("schedule push failed",
					zap.String("platform", ch.Platform), zap.Error(err))
			} else {
				pushed++
			}
		}
		if len(channels) > 0 && pushed == 0 {
			s.logger.Warn("schedule push failed on all channels", zap.String("bot", b.Name))
		}
	}
	return result, nil
}

// truncate 截断字符串
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
