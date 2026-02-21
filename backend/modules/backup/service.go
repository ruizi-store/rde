package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service 备份服务
type Service struct {
	logger    *zap.Logger
	db        *gorm.DB
	dataDir   string
	backupDir string

	// 运行中的备份任务
	runningMu sync.RWMutex
	running   map[string]*runningTask

	// 备份目标管理
	targets map[TargetType]Target

	// 通知回调
	notifyCallback func(title, content string, isError bool)

	// 加密密码（从配置获取）
	encryptionPassword string
}

// runningTask 运行中的任务
type runningTask struct {
	RecordID  string
	Cancel    chan struct{}
	Progress  int
	Message   string
	StartedAt time.Time
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, db *gorm.DB, dataDir string) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	s := &Service{
		logger:    logger,
		db:        db,
		dataDir:   dataDir,
		backupDir: filepath.Join(dataDir, "backups"),
		running:   make(map[string]*runningTask),
		targets:   make(map[TargetType]Target),
	}

	// 注册所有目标适配器
	s.targets[TargetTypeLocal] = &LocalTarget{}
	s.targets[TargetTypeWebDAV] = &WebDAVTarget{}
	s.targets[TargetTypeS3] = &S3Target{}
	s.targets[TargetTypeSFTP] = &SFTPTarget{}

	return s
}

// SetNotifyCallback 设置通知回调
func (s *Service) SetNotifyCallback(callback func(title, content string, isError bool)) {
	s.notifyCallback = callback
}

// SetEncryptionPassword 设置加密密码
func (s *Service) SetEncryptionPassword(password string) {
	s.encryptionPassword = password
}

// notify 发送通知
func (s *Service) notify(title, content string, isError bool) {
	if s.notifyCallback != nil {
		s.notifyCallback(title, content, isError)
	}
}

// RegisterTarget 注册备份目标
func (s *Service) RegisterTarget(t TargetType, target Target) {
	s.targets[t] = target
}

// GetOverview 获取备份概览
func (s *Service) GetOverview() (*BackupOverview, error) {
	var overview BackupOverview

	// 统计任务数
	var totalTasks, enabledTasks, totalRecords, successCount, failedCount int64
	s.db.Model(&BackupTaskModel{}).Count(&totalTasks)
	s.db.Model(&BackupTaskModel{}).Where("enabled = ?", true).Count(&enabledTasks)
	overview.TotalTasks = int(totalTasks)
	overview.EnabledTasks = int(enabledTasks)

	// 统计记录数
	s.db.Model(&BackupRecordModel{}).Count(&totalRecords)
	s.db.Model(&BackupRecordModel{}).Where("status = ?", BackupStatusSuccess).Count(&successCount)
	s.db.Model(&BackupRecordModel{}).Where("status = ?", BackupStatusFailed).Count(&failedCount)
	overview.TotalRecords = int(totalRecords)
	overview.SuccessCount = int(successCount)
	overview.FailedCount = int(failedCount)

	// 统计总大小
	s.db.Model(&BackupRecordModel{}).Select("COALESCE(SUM(size), 0)").Where("status = ?", BackupStatusSuccess).Scan(&overview.TotalSize)

	// 最近备份时间
	var lastRecord BackupRecordModel
	if err := s.db.Where("status = ?", BackupStatusSuccess).Order("completed_at DESC").First(&lastRecord).Error; err == nil {
		overview.LastBackupAt = lastRecord.CompletedAt
	}

	// 下次备份时间
	var nextTask BackupTaskModel
	if err := s.db.Where("enabled = ? AND next_run_at IS NOT NULL", true).Order("next_run_at ASC").First(&nextTask).Error; err == nil {
		overview.NextBackupAt = nextTask.NextRunAt
	}

	return &overview, nil
}

// ListTasks 获取任务列表
func (s *Service) ListTasks(req *ListTasksRequest) ([]*BackupTask, int64, error) {
	var models []BackupTaskModel
	var total int64

	query := s.db.Model(&BackupTaskModel{})

	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.Enabled != nil {
		query = query.Where("enabled = ?", *req.Enabled)
	}

	query.Count(&total)

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	tasks := make([]*BackupTask, len(models))
	for i := range models {
		tasks[i] = models[i].ToBackupTask()
	}

	return tasks, total, nil
}

// GetTask 获取任务详情
func (s *Service) GetTask(id string) (*BackupTask, error) {
	var model BackupTaskModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return model.ToBackupTask(), nil
}

// CreateTask 创建任务
func (s *Service) CreateTask(req *CreateTaskRequest) (*BackupTask, error) {
	model := &BackupTaskModel{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		Type:         string(req.Type),
		Sources:      toJSONString(req.Sources),
		TargetType:   string(req.TargetType),
		TargetConfig: req.TargetConfig,
		Schedule:     req.Schedule,
		Retention:    req.Retention,
		Compression:  req.Compression,
		Encryption:   req.Encryption,
		Enabled:      true,
	}

	if model.Retention <= 0 {
		model.Retention = 7
	}

	if err := s.db.Create(model).Error; err != nil {
		return nil, err
	}

	return model.ToBackupTask(), nil
}

// UpdateTask 更新任务
func (s *Service) UpdateTask(id string, req *UpdateTaskRequest) (*BackupTask, error) {
	var model BackupTaskModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if len(req.Sources) > 0 {
		updates["sources"] = toJSONString(req.Sources)
	}
	if req.TargetType != "" {
		updates["target_type"] = string(req.TargetType)
	}
	if req.TargetConfig != "" {
		updates["target_config"] = req.TargetConfig
	}
	if req.Schedule != "" {
		updates["schedule"] = req.Schedule
	}
	if req.Retention != nil {
		updates["retention"] = *req.Retention
	}
	if req.Compression != nil {
		updates["compression"] = *req.Compression
	}
	if req.Encryption != nil {
		updates["encryption"] = *req.Encryption
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := s.db.Model(&model).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetTask(id)
}

// DeleteTask 删除任务
func (s *Service) DeleteTask(id string) error {
	return s.db.Delete(&BackupTaskModel{}, "id = ?", id).Error
}

// RunTask 立即执行备份任务
func (s *Service) RunTask(taskID string) (*BackupRecord, error) {
	task, err := s.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// 创建备份记录
	record := &BackupRecordModel{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		Type:      string(task.Type),
		Status:    string(BackupStatusRunning),
		Progress:  0,
		Message:   "准备中...",
		StartedAt: time.Now(),
	}

	if err := s.db.Create(record).Error; err != nil {
		return nil, err
	}

	// 添加到运行列表
	s.runningMu.Lock()
	s.running[record.ID] = &runningTask{
		RecordID:  record.ID,
		Cancel:    make(chan struct{}),
		StartedAt: time.Now(),
	}
	s.runningMu.Unlock()

	// 异步执行备份
	go s.executeBackup(task, record)

	return record.ToBackupRecord(), nil
}

// executeBackup 执行备份
func (s *Service) executeBackup(task *BackupTask, record *BackupRecordModel) {
	defer func() {
		s.runningMu.Lock()
		delete(s.running, record.ID)
		s.runningMu.Unlock()
	}()

	s.updateRecordProgress(record.ID, 5, "正在准备备份...")

	// 获取目标适配器
	target, ok := s.targets[task.TargetType]
	if !ok {
		s.failRecord(record.ID, "不支持的备份目标类型: "+string(task.TargetType))
		return
	}

	// 解析目标配置
	if err := target.Configure(task.TargetConfig); err != nil {
		s.failRecord(record.ID, "配置备份目标失败: "+err.Error())
		return
	}

	s.updateRecordProgress(record.ID, 10, "正在打包文件...")

	// 打包备份文件
	var tempPath string
	var fileCount int
	var backupName string
	{
		timestamp := time.Now().Format("20060102-150405")
		backupName = fmt.Sprintf("%s-%s.tar.gz", task.Name, timestamp)
		tempPath = filepath.Join(s.backupDir, "temp", backupName)

		if err := os.MkdirAll(filepath.Dir(tempPath), 0755); err != nil {
			s.failRecord(record.ID, "创建临时目录失败: "+err.Error())
			return
		}

		var err error
		fileCount, err = s.createArchive(task.Sources, tempPath, task.Compression, record.ID)
		if err != nil {
			s.failRecord(record.ID, "创建备份归档失败: "+err.Error())
			os.Remove(tempPath)
			return
		}
	}

	s.updateRecordProgress(record.ID, 70, "正在计算校验和...")

	// 计算校验和
	checksum, err := s.calculateChecksum(tempPath)
	if err != nil {
		s.failRecord(record.ID, "计算校验和失败: "+err.Error())
		os.Remove(tempPath)
		return
	}

	// 获取文件大小
	stat, err := os.Stat(tempPath)
	if err != nil {
		s.failRecord(record.ID, "获取文件信息失败: "+err.Error())
		os.Remove(tempPath)
		return
	}

	s.updateRecordProgress(record.ID, 80, "正在上传到目标...")

	// 上传到目标
	remotePath, err := target.Upload(tempPath, backupName, func(progress int) {
		s.updateRecordProgress(record.ID, 80+progress/5, "正在上传...")
	})
	if err != nil {
		s.failRecord(record.ID, "上传失败: "+err.Error())
		os.Remove(tempPath)
		return
	}

	// 删除临时文件
	os.Remove(tempPath)

	// 更新记录
	now := time.Now()
	s.db.Model(record).Updates(map[string]interface{}{
		"status":       string(BackupStatusSuccess),
		"progress":     100,
		"message":      "备份完成",
		"size":         stat.Size(),
		"file_count":   fileCount,
		"file_path":    remotePath,
		"checksum":     checksum,
		"completed_at": &now,
	})

	// 更新任务最后运行时间
	s.db.Model(&BackupTaskModel{}).Where("id = ?", task.ID).Update("last_run_at", &now)

	// 清理旧备份
	s.cleanOldBackups(task)

	// 发送通知
	s.notify(
		"备份完成",
		fmt.Sprintf("任务 [%s] 备份成功，大小: %s", task.Name, formatSize(stat.Size())),
		false,
	)

	s.logger.Info("Backup completed",
		zap.String("task_id", task.ID),
		zap.String("record_id", record.ID),
		zap.Int64("size", stat.Size()),
	)
}

// createArchive 创建归档文件
func (s *Service) createArchive(sources []string, destPath string, compress bool, recordID string) (int, error) {
	file, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var writer io.Writer = file
	var gzWriter *gzip.Writer

	if compress {
		gzWriter = gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter
	}

	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()

	fileCount := 0
	totalFiles := 0

	// 先统计总文件数
	for _, source := range sources {
		filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				totalFiles++
			}
			return nil
		})
	}

	// 遍历所有源路径
	for _, source := range sources {
		err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 创建 tar header
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			// 使用相对路径
			relPath, err := filepath.Rel(filepath.Dir(source), path)
			if err != nil {
				relPath = path
			}
			header.Name = relPath

			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			// 写入文件内容
			if !info.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()

				if _, err := io.Copy(tarWriter, f); err != nil {
					return err
				}

				fileCount++

				// 更新进度
				if totalFiles > 0 {
					progress := 10 + (fileCount * 60 / totalFiles)
					s.updateRecordProgress(recordID, progress, fmt.Sprintf("正在打包: %s", info.Name()))
				}
			}

			return nil
		})

		if err != nil {
			return fileCount, err
		}
	}

	return fileCount, nil
}

// calculateChecksum 计算文件校验和
func (s *Service) calculateChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// updateRecordProgress 更新记录进度
func (s *Service) updateRecordProgress(recordID string, progress int, message string) {
	s.db.Model(&BackupRecordModel{}).Where("id = ?", recordID).Updates(map[string]interface{}{
		"progress": progress,
		"message":  message,
	})

	s.runningMu.Lock()
	if task, ok := s.running[recordID]; ok {
		task.Progress = progress
		task.Message = message
	}
	s.runningMu.Unlock()
}

// failRecord 标记记录失败
func (s *Service) failRecord(recordID string, errMsg string) {
	now := time.Now()
	s.db.Model(&BackupRecordModel{}).Where("id = ?", recordID).Updates(map[string]interface{}{
		"status":       string(BackupStatusFailed),
		"error":        errMsg,
		"completed_at": &now,
	})

	// 发送失败通知
	s.notify("备份失败", errMsg, true)

	s.logger.Error("Backup failed", zap.String("record_id", recordID), zap.String("error", errMsg))
}

// formatSize 格式化文件大小
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// cleanOldBackups 清理旧备份
func (s *Service) cleanOldBackups(task *BackupTask) {
	if task.Retention <= 0 {
		return
	}

	var records []BackupRecordModel
	s.db.Where("task_id = ? AND status = ?", task.ID, BackupStatusSuccess).
		Order("completed_at DESC").
		Offset(task.Retention).
		Find(&records)

	target, ok := s.targets[task.TargetType]
	if !ok {
		return
	}
	target.Configure(task.TargetConfig)

	for _, r := range records {
		// 删除远程文件
		if r.FilePath != "" {
			if err := target.Delete(r.FilePath); err != nil {
				s.logger.Warn("Failed to delete old backup file",
					zap.String("path", r.FilePath),
					zap.Error(err))
			}
		}

		// 删除记录
		s.db.Delete(&r)
	}
}

// ListRecords 获取备份记录
func (s *Service) ListRecords(req *ListRecordsRequest) ([]*BackupRecord, int64, error) {
	var models []BackupRecordModel
	var total int64

	query := s.db.Model(&BackupRecordModel{})

	if req.TaskID != "" {
		query = query.Where("task_id = ?", req.TaskID)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	query.Count(&total)

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	err := query.Order("started_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	records := make([]*BackupRecord, len(models))
	for i := range models {
		records[i] = models[i].ToBackupRecord()
		// 关联任务名称
		var task BackupTaskModel
		if s.db.First(&task, "id = ?", records[i].TaskID).Error == nil {
			records[i].TaskName = task.Name
		}
	}

	return records, total, nil
}

// GetRecord 获取记录详情
func (s *Service) GetRecord(id string) (*BackupRecord, error) {
	var model BackupRecordModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		return nil, err
	}
	record := model.ToBackupRecord()

	// 关联任务名称
	var task BackupTaskModel
	if s.db.First(&task, "id = ?", record.TaskID).Error == nil {
		record.TaskName = task.Name
	}

	return record, nil
}

// DeleteRecord 删除记录
func (s *Service) DeleteRecord(id string) error {
	var record BackupRecordModel
	if err := s.db.First(&record, "id = ?", id).Error; err != nil {
		return err
	}

	// 获取任务信息以删除远程文件
	var task BackupTaskModel
	if s.db.First(&task, "id = ?", record.TaskID).Error == nil {
		if target, ok := s.targets[TargetType(task.TargetType)]; ok {
			target.Configure(task.TargetConfig)
			target.Delete(record.FilePath)
		}
	}

	return s.db.Delete(&record).Error
}

// Restore 执行还原
func (s *Service) Restore(req *RestoreRequest) (*RestoreStatus, error) {
	// 获取备份记录
	record, err := s.GetRecord(req.RecordID)
	if err != nil {
		return nil, fmt.Errorf("备份记录不存在: %w", err)
	}

	if record.Status != BackupStatusSuccess {
		return nil, fmt.Errorf("该备份未成功完成，无法还原")
	}

	// 创建还原记录
	restore := &RestoreRecordModel{
		ID:        uuid.New().String(),
		BackupID:  req.RecordID,
		TargetPath: req.TargetPath,
		Status:    string(BackupStatusRunning),
		Progress:  0,
		Message:   "准备中...",
		StartedAt: time.Now(),
	}

	if err := s.db.Create(restore).Error; err != nil {
		return nil, err
	}

	// 异步执行还原
	go s.executeRestore(record, restore, req)

	return restore.ToRestoreStatus(), nil
}

// executeRestore 执行还原操作
func (s *Service) executeRestore(backup *BackupRecord, restore *RestoreRecordModel, req *RestoreRequest) {
	// 获取任务信息
	task, err := s.GetTask(backup.TaskID)
	if err != nil {
		s.failRestore(restore.ID, "获取任务信息失败: "+err.Error())
		return
	}

	// 获取目标适配器
	target, ok := s.targets[task.TargetType]
	if !ok {
		s.failRestore(restore.ID, "不支持的备份目标类型")
		return
	}
	target.Configure(task.TargetConfig)

	s.updateRestoreProgress(restore.ID, 10, "正在下载备份文件...")

	// 下载备份文件
	tempPath := filepath.Join(s.backupDir, "temp", "restore-"+restore.ID+".tar.gz")
	if err := os.MkdirAll(filepath.Dir(tempPath), 0755); err != nil {
		s.failRestore(restore.ID, "创建临时目录失败: "+err.Error())
		return
	}

	if err := target.Download(backup.FilePath, tempPath, func(progress int) {
		s.updateRestoreProgress(restore.ID, 10+progress/2, "正在下载...")
	}); err != nil {
		s.failRestore(restore.ID, "下载失败: "+err.Error())
		return
	}

	s.updateRestoreProgress(restore.ID, 60, "正在解压文件...")

	// 确定还原目标路径
	targetPath := req.TargetPath
	if targetPath == "" && len(task.Sources) > 0 {
		targetPath = filepath.Dir(task.Sources[0])
	}

	// 解压还原（支持选择性还原）
	if err := s.extractArchive(tempPath, targetPath, req.Overwrite, req.SelectedItems, restore.ID); err != nil {
		s.failRestore(restore.ID, "解压失败: "+err.Error())
		os.Remove(tempPath)
		return
	}

	// 清理临时文件
	os.Remove(tempPath)

	// 更新状态
	now := time.Now()
	s.db.Model(restore).Updates(map[string]interface{}{
		"status":       string(BackupStatusSuccess),
		"progress":     100,
		"message":      "还原完成",
		"completed_at": &now,
	})

	s.logger.Info("Restore completed", zap.String("restore_id", restore.ID))
}

// extractArchive 解压归档（支持选择性还原）
func (s *Service) extractArchive(archivePath, destPath string, overwrite bool, selectedItems []string, restoreID string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f

	// 检测并处理 gzip
	if strings.HasSuffix(archivePath, ".gz") {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// 选择性还原：如果指定了选择项，只还原匹配的文件
		if len(selectedItems) > 0 && !s.matchSelectedItem(header.Name, selectedItems) {
			continue
		}

		targetPath := filepath.Join(destPath, header.Name)

		s.updateRestoreProgress(restoreID, -1, "正在解压: "+header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// 检查文件是否存在
			if _, err := os.Stat(targetPath); err == nil && !overwrite {
				continue // 跳过已存在的文件
			}

			// 确保目录存在
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			// 恢复文件权限
			os.Chmod(targetPath, os.FileMode(header.Mode))
		}
	}

	return nil
}

// matchSelectedItem 检查文件是否匹配选择项
func (s *Service) matchSelectedItem(filePath string, selectedItems []string) bool {
	for _, item := range selectedItems {
		// 精确匹配或前缀匹配（目录）
		if filePath == item || strings.HasPrefix(filePath, item+"/") || strings.HasPrefix(item, filePath+"/") {
			return true
		}
	}
	return false
}

// updateRestoreProgress 更新还原进度
func (s *Service) updateRestoreProgress(restoreID string, progress int, message string) {
	updates := map[string]interface{}{"message": message}
	if progress >= 0 {
		updates["progress"] = progress
	}
	s.db.Model(&RestoreRecordModel{}).Where("id = ?", restoreID).Updates(updates)
}

// failRestore 标记还原失败
func (s *Service) failRestore(restoreID string, errMsg string) {
	now := time.Now()
	s.db.Model(&RestoreRecordModel{}).Where("id = ?", restoreID).Updates(map[string]interface{}{
		"status":       string(BackupStatusFailed),
		"error":        errMsg,
		"completed_at": &now,
	})
	s.logger.Error("Restore failed", zap.String("restore_id", restoreID), zap.String("error", errMsg))
}

// GetRestoreStatus 获取还原状态
func (s *Service) GetRestoreStatus(id string) (*RestoreStatus, error) {
	var model RestoreRecordModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return model.ToRestoreStatus(), nil
}

// TestTarget 测试目标连接
func (s *Service) TestTarget(req *TargetTestRequest) *TargetTestResponse {
	target, ok := s.targets[req.Type]
	if !ok {
		return &TargetTestResponse{
			Success: false,
			Message: "不支持的目标类型: " + string(req.Type),
		}
	}

	if err := target.Configure(req.Config); err != nil {
		return &TargetTestResponse{
			Success: false,
			Message: "配置解析失败: " + err.Error(),
		}
	}

	return target.Test()
}

// GetExportableConfigs 获取可导出的配置项
func (s *Service) GetExportableConfigs() []ExportableConfig {
	return []ExportableConfig{
		{ID: "users", Name: "用户账户", Description: "用户信息和权限设置", Category: "system"},
		{ID: "docker", Name: "Docker配置", Description: "容器和编排配置", Category: "app"},
		{ID: "samba", Name: "Samba共享", Description: "文件共享配置", Category: "app"},
		{ID: "proxy", Name: "反向代理", Description: "代理规则配置", Category: "system"},
		{ID: "ddns", Name: "DDNS配置", Description: "动态DNS配置", Category: "system"},
		{ID: "notification", Name: "通知设置", Description: "通知渠道配置", Category: "system"},
		{ID: "settings", Name: "系统设置", Description: "全局偏好设置", Category: "system"},
	}
}

// ExportConfig 导出配置
func (s *Service) ExportConfig(req *ConfigExportRequest) ([]byte, error) {
	config := make(map[string]interface{})

	for _, item := range req.IncludeItems {
		switch item {
		case "settings":
			// 导出系统设置
			var settings []map[string]interface{}
			s.db.Table("settings").Find(&settings)
			config["settings"] = settings
		case "users":
			// 导出用户（排除密码）
			var users []map[string]interface{}
			s.db.Table("users").Select("id, username, email, role, created_at").Find(&users)
			config["users"] = users
			// 可以继续添加其他配置项...
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}

	// TODO: 如果需要加密，在这里处理

	return data, nil
}

// ImportConfig 导入配置
func (s *Service) ImportConfig(data []byte, req *ConfigImportRequest) error {
	// TODO: 如果加密了，先解密

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("配置文件格式错误: %w", err)
	}

	// TODO: 实现各配置项的导入逻辑

	return nil
}
