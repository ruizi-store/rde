// Package photos 照片管理模块
package photos

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Scanner 目录扫描器
type Scanner struct {
	logger      *zap.Logger
	db          *gorm.DB
	service     *Service
	config      Config
	indexer     *Indexer
	progress    sync.Map // libraryID -> *ScanProgress
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	indexQueue  chan *Photo
	indexWg     sync.WaitGroup
}

// NewScanner 创建扫描器
func NewScanner(logger *zap.Logger, db *gorm.DB, service *Service, config Config) *Scanner {
	s := &Scanner{
		logger:     logger,
		db:         db,
		service:    service,
		config:     config,
		indexer:    NewIndexer(logger, db, service),
		indexQueue: make(chan *Photo, 1000), // 缓冲队列
	}
	return s
}

// Start 启动扫描器
func (s *Scanner) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// 启动索引工作者池
	workerCount := s.config.MaxScanWorkers
	if workerCount <= 0 {
		workerCount = 4
	}
	for i := 0; i < workerCount; i++ {
		s.indexWg.Add(1)
		go s.indexWorker(ctx, i)
	}

	// 启动定时扫描
	s.wg.Add(1)
	go s.runScheduler(ctx)

	s.logger.Info("scanner started",
		zap.Int("interval_minutes", s.config.ScanInterval),
		zap.Int("workers", workerCount))
}

// indexWorker 索引工作者
func (s *Scanner) indexWorker(ctx context.Context, id int) {
	defer s.indexWg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case photo, ok := <-s.indexQueue:
			if !ok {
				return
			}
			if err := s.indexer.IndexPhoto(photo); err != nil {
				s.logger.Debug("index worker failed",
					zap.Int("worker", id),
					zap.String("photo", photo.ID),
					zap.Error(err))
			}
		}
	}
}

// Stop 停止扫描器
func (s *Scanner) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	close(s.indexQueue)
	s.indexWg.Wait()
	s.wg.Wait()
	s.logger.Info("scanner stopped")
}

// runScheduler 运行定时调度
func (s *Scanner) runScheduler(ctx context.Context) {
	defer s.wg.Done()

	// 启动时立即扫描一次
	s.scanAllLibraries(ctx)

	interval := time.Duration(s.config.ScanInterval) * time.Minute
	if interval < time.Minute {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scanAllLibraries(ctx)
		}
	}
}

// scanAllLibraries 扫描所有图库
func (s *Scanner) scanAllLibraries(ctx context.Context) {
	var libraries []Library
	if err := s.db.Where("scan_enabled = ?", true).Find(&libraries).Error; err != nil {
		s.logger.Error("failed to load libraries", zap.Error(err))
		return
	}

	for _, lib := range libraries {
		select {
		case <-ctx.Done():
			return
		default:
			s.ScanLibrary(ctx, lib.ID)
		}
	}
}

// ScanLibrary 扫描指定图库
func (s *Scanner) ScanLibrary(ctx context.Context, libraryID string) error {
	library, err := s.service.GetLibrary(libraryID)
	if err != nil {
		return err
	}

	// 检查是否正在扫描
	if progress, ok := s.progress.Load(libraryID); ok {
		if p, ok := progress.(*ScanProgress); ok && p.Status == "scanning" {
			s.logger.Debug("library scan already in progress", zap.String("library_id", libraryID))
			return nil
		}
	}

	// 初始化进度
	progress := &ScanProgress{
		LibraryID: libraryID,
		Status:    "scanning",
		StartedAt: time.Now().Format(time.RFC3339),
	}
	s.progress.Store(libraryID, progress)

	s.logger.Info("starting library scan",
		zap.String("library_id", libraryID),
		zap.String("path", library.Path))

	// 扫描目录
	var totalFiles, scannedFiles, indexedFiles, failedFiles int32

	err = filepath.WalkDir(library.Path, func(path string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			s.logger.Debug("walk error", zap.String("path", path), zap.Error(err))
			return nil // 继续扫描
		}

		if d.IsDir() {
			// 跳过隐藏目录
			if len(d.Name()) > 0 && d.Name()[0] == '.' {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否为媒体文件
		isMedia, mediaType := IsMediaFile(path)
		if !isMedia {
			return nil
		}

		atomic.AddInt32(&totalFiles, 1)

		// 更新进度
		progress.TotalFiles = int(atomic.LoadInt32(&totalFiles))
		progress.ScannedFiles = int(atomic.LoadInt32(&scannedFiles))

		// 检查是否已存在
		var count int64
		s.db.Model(&Photo{}).Where("path = ?", path).Count(&count)
		if count > 0 {
			atomic.AddInt32(&scannedFiles, 1)
			return nil
		}

		// 获取文件信息
		info, err := d.Info()
		if err != nil {
			s.logger.Debug("failed to get file info", zap.String("path", path), zap.Error(err))
			atomic.AddInt32(&failedFiles, 1)
			return nil
		}

		// 创建照片记录
		photo := &Photo{
			ID:        uuid.New().String(),
			LibraryID: libraryID,
			Path:      path,
			Filename:  d.Name(),
			Size:      info.Size(),
			Type:      mediaType,
			Status:    "pending",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := s.db.Create(photo).Error; err != nil {
			s.logger.Debug("failed to create photo record", zap.String("path", path), zap.Error(err))
			atomic.AddInt32(&failedFiles, 1)
			return nil
		}

		atomic.AddInt32(&scannedFiles, 1)

		// 发送到索引队列
		select {
		case s.indexQueue <- photo:
			atomic.AddInt32(&indexedFiles, 1)
		default:
			// 队列满，记录警告
			s.logger.Debug("index queue full, skipping", zap.String("path", path))
		}

		return nil
	})

	// 更新进度
	progress.Status = "completed"
	progress.TotalFiles = int(totalFiles)
	progress.ScannedFiles = int(scannedFiles)
	progress.IndexedFiles = int(indexedFiles)
	progress.FailedFiles = int(failedFiles)
	progress.CompletedAt = time.Now().Format(time.RFC3339)
	if err != nil {
		progress.Status = "failed"
		progress.Error = err.Error()
	}

	// 更新图库统计
	s.updateLibraryStats(libraryID)

	// 更新最后扫描时间
	now := time.Now()
	s.db.Model(&Library{}).Where("id = ?", libraryID).Updates(map[string]interface{}{
		"last_scan_at": now,
		"updated_at":   now,
	})

	s.logger.Info("library scan completed",
		zap.String("library_id", libraryID),
		zap.Int32("total", totalFiles),
		zap.Int32("scanned", scannedFiles),
		zap.Int32("indexed", indexedFiles),
		zap.Int32("failed", failedFiles))

	return err
}

// GetProgress 获取扫描进度
func (s *Scanner) GetProgress(libraryID string) *ScanProgress {
	if progress, ok := s.progress.Load(libraryID); ok {
		if p, ok := progress.(*ScanProgress); ok {
			return p
		}
	}
	return &ScanProgress{
		LibraryID: libraryID,
		Status:    "idle",
	}
}

// IsScanning 检查是否正在扫描
func (s *Scanner) IsScanning(libraryID string) bool {
	progress := s.GetProgress(libraryID)
	return progress.Status == "scanning"
}

// updateLibraryStats 更新图库统计
func (s *Scanner) updateLibraryStats(libraryID string) {
	var photoCount, videoCount int64
	var totalSize int64

	s.db.Model(&Photo{}).Where("library_id = ? AND type = ? AND is_deleted = ?", libraryID, "photo", false).Count(&photoCount)
	s.db.Model(&Photo{}).Where("library_id = ? AND type = ? AND is_deleted = ?", libraryID, "video", false).Count(&videoCount)
	s.db.Model(&Photo{}).Where("library_id = ? AND is_deleted = ?", libraryID, false).Select("COALESCE(SUM(size), 0)").Scan(&totalSize)

	s.db.Model(&Library{}).Where("id = ?", libraryID).Updates(map[string]interface{}{
		"photo_count": photoCount,
		"video_count": videoCount,
		"total_size":  totalSize,
		"updated_at":  time.Now(),
	})
}

// CleanupDeletedFiles 清理已删除的文件记录
func (s *Scanner) CleanupDeletedFiles(libraryID string) (int64, error) {
	var photos []Photo
	if err := s.db.Where("library_id = ?", libraryID).Find(&photos).Error; err != nil {
		return 0, err
	}

	var deleted int64
	for _, photo := range photos {
		if _, err := os.Stat(photo.Path); os.IsNotExist(err) {
			if err := s.db.Delete(&photo).Error; err == nil {
				deleted++
			}
		}
	}

	if deleted > 0 {
		s.updateLibraryStats(libraryID)
		s.logger.Info("cleaned up deleted files",
			zap.String("library_id", libraryID),
			zap.Int64("count", deleted))
	}

	return deleted, nil
}
