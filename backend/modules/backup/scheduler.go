package backup

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Scheduler 备份调度器
type Scheduler struct {
	logger  *zap.Logger
	service *Service
	cron    *cron.Cron
	jobs    map[string]cron.EntryID
	mu      sync.RWMutex
}

// NewScheduler 创建调度器
func NewScheduler(logger *zap.Logger, service *Service) *Scheduler {
	return &Scheduler{
		logger:  logger,
		service: service,
		jobs:    make(map[string]cron.EntryID),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.cron = cron.New(cron.WithSeconds())

	// 加载所有启用的定时任务
	tasks, _, err := s.service.ListTasks(&ListTasksRequest{
		PageSize: 1000,
	})
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task.Enabled && task.Schedule != "" {
			s.scheduleTask(task)
		}
	}

	s.cron.Start()
	s.logger.Info("Backup scheduler started", zap.Int("tasks", len(s.jobs)))
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
	s.logger.Info("Backup scheduler stopped")
}

// scheduleTask 调度任务
func (s *Scheduler) scheduleTask(task *BackupTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 移除旧的调度
	if entryID, ok := s.jobs[task.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, task.ID)
	}

	if task.Schedule == "" || !task.Enabled {
		return
	}

	taskID := task.ID
	entryID, err := s.cron.AddFunc(task.Schedule, func() {
		s.runScheduledTask(taskID)
	})

	if err != nil {
		s.logger.Error("Failed to schedule task",
			zap.String("task_id", task.ID),
			zap.String("schedule", task.Schedule),
			zap.Error(err))
		return
	}

	s.jobs[task.ID] = entryID

	// 更新下次运行时间
	entry := s.cron.Entry(entryID)
	nextRun := entry.Next
	s.service.db.Model(&BackupTaskModel{}).Where("id = ?", task.ID).Update("next_run_at", &nextRun)

	s.logger.Info("Task scheduled",
		zap.String("task_id", task.ID),
		zap.String("schedule", task.Schedule),
		zap.Time("next_run", nextRun))
}

// runScheduledTask 执行定时任务
func (s *Scheduler) runScheduledTask(taskID string) {
	s.logger.Info("Running scheduled backup", zap.String("task_id", taskID))

	record, err := s.service.RunTask(taskID)
	if err != nil {
		s.logger.Error("Failed to run scheduled backup",
			zap.String("task_id", taskID),
			zap.Error(err))
		return
	}

	s.logger.Info("Scheduled backup started",
		zap.String("task_id", taskID),
		zap.String("record_id", record.ID))

	// 更新下次运行时间
	s.mu.RLock()
	if entryID, ok := s.jobs[taskID]; ok {
		entry := s.cron.Entry(entryID)
		nextRun := entry.Next
		s.service.db.Model(&BackupTaskModel{}).Where("id = ?", taskID).Update("next_run_at", &nextRun)
	}
	s.mu.RUnlock()
}

// UpdateTask 更新任务调度
func (s *Scheduler) UpdateTask(task *BackupTask) {
	s.scheduleTask(task)
}

// RemoveTask 移除任务调度
func (s *Scheduler) RemoveTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.jobs[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, taskID)
		s.logger.Info("Task unscheduled", zap.String("task_id", taskID))
	}
}

// GetNextRun 获取任务下次运行时间
func (s *Scheduler) GetNextRun(taskID string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if entryID, ok := s.jobs[taskID]; ok {
		entry := s.cron.Entry(entryID)
		return &entry.Next
	}
	return nil
}
