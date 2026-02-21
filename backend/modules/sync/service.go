package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusHandler "github.com/tus/tusd/v2/pkg/handler"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service 文件同步服务
type Service struct {
	logger     *zap.Logger
	db         *gorm.DB
	dataDir    string
	uploadDir  string
	filesDir   string
	tusStore   filestore.FileStore
	tusHandler *tusHandler.Handler
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, db *gorm.DB, dataDir string) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		logger:  logger,
		db:      db,
		dataDir: dataDir,
	}
}

// Init 初始化服务
func (s *Service) Init() error {
	// 设置目录
	s.uploadDir = filepath.Join(s.dataDir, "sync", "uploads")
	s.filesDir = filepath.Join(s.dataDir, "sync", "files")

	// 创建目录
	for _, dir := range []string{s.uploadDir, s.filesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}

	// 初始化数据库表
	if err := s.initDB(); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 初始化 TUS
	if err := s.initTUS(); err != nil {
		return fmt.Errorf("初始化 TUS 失败: %w", err)
	}

	return nil
}

// initDB 初始化数据库表
func (s *Service) initDB() error {
	// 自动迁移
	return s.db.AutoMigrate(&SyncFileModel{}, &UploadSessionModel{})
}

// SyncFileModel 数据库模型
type SyncFileModel struct {
	ID        string    `gorm:"primaryKey"`
	Filename  string    `gorm:"not null"`
	Size      int64     `gorm:"not null"`
	MimeType  string    `gorm:"column:mime_type"`
	SHA256    string    `gorm:"column:sha256"`
	Path      string    `gorm:"not null"`
	Status    string    `gorm:"default:completed"`
	UserID    string    `gorm:"column:user_id;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (SyncFileModel) TableName() string {
	return "sync_files"
}

// UploadSessionModel 上传会话数据库模型
type UploadSessionModel struct {
	ID        string    `gorm:"primaryKey"`
	FileID    string    `gorm:"column:file_id"`
	Filename  string    `gorm:"not null"`
	Size      int64     `gorm:"not null"`
	Offset    int64     `gorm:"default:0"`
	Metadata  string    `gorm:"type:text"`
	UserID    string    `gorm:"column:user_id;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	ExpiresAt time.Time
}

func (UploadSessionModel) TableName() string {
	return "upload_sessions"
}

// initTUS 初始化 TUS 上传处理器
func (s *Service) initTUS() error {
	s.tusStore = filestore.New(s.uploadDir)

	composer := tusHandler.NewStoreComposer()
	s.tusStore.UseIn(composer)

	config := tusHandler.Config{
		BasePath:                "/api/v1/sync/upload/",
		StoreComposer:           composer,
		MaxSize:                 50 * 1024 * 1024 * 1024, // 50GB
		NotifyCompleteUploads:   true,
		NotifyCreatedUploads:    true,
		RespectForwardedHeaders: true,
	}

	handler, err := tusHandler.NewHandler(config)
	if err != nil {
		return err
	}

	s.tusHandler = handler

	// 处理上传完成事件
	go s.handleTusEvents()

	return nil
}

// handleTusEvents 处理 TUS 事件
func (s *Service) handleTusEvents() {
	for {
		select {
		case event := <-s.tusHandler.CompleteUploads:
			s.onUploadComplete(event)
		case event := <-s.tusHandler.CreatedUploads:
			s.onUploadCreated(event)
		}
	}
}

// onUploadCreated 上传创建时
func (s *Service) onUploadCreated(event tusHandler.HookEvent) {
	info := event.Upload

	session := &UploadSessionModel{
		ID:        info.ID,
		Filename:  info.MetaData["filename"],
		Size:      info.Size,
		Offset:    info.Offset,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := s.db.Create(session).Error; err != nil {
		s.logger.Error("保存上传会话失败", zap.Error(err))
	}
}

// onUploadComplete 上传完成时
func (s *Service) onUploadComplete(event tusHandler.HookEvent) {
	info := event.Upload

	// 读取上传的文件
	srcPath := filepath.Join(s.uploadDir, info.ID)

	// 计算 SHA256
	sha256Hash, err := s.calculateSHA256(srcPath)
	if err != nil {
		s.logger.Error("计算文件哈希失败", zap.Error(err))
		return
	}

	// 移动到 files 目录
	fileID := uuid.New().String()
	filename := info.MetaData["filename"]
	ext := filepath.Ext(filename)
	dstPath := filepath.Join(s.filesDir, fileID+ext)

	if err := os.Rename(srcPath, dstPath); err != nil {
		s.logger.Error("移动文件失败", zap.Error(err))
		return
	}

	// 删除 .info 文件
	os.Remove(srcPath + ".info")

	// 保存到数据库
	file := &SyncFileModel{
		ID:        fileID,
		Filename:  filename,
		Size:      info.Size,
		MimeType:  info.MetaData["filetype"],
		SHA256:    sha256Hash,
		Path:      dstPath,
		Status:    "completed",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Create(file).Error; err != nil {
		s.logger.Error("保存文件记录失败", zap.Error(err))
		return
	}

	// 删除上传会话
	s.db.Delete(&UploadSessionModel{}, "id = ?", info.ID)

	s.logger.Info("文件上传完成",
		zap.String("id", fileID),
		zap.String("filename", filename),
		zap.Int64("size", info.Size))
}

// calculateSHA256 计算文件 SHA256
func (s *Service) calculateSHA256(path string) (string, error) {
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

// GetTusHandler 获取 TUS 处理器
func (s *Service) GetTusHandler() *tusHandler.Handler {
	return s.tusHandler
}

// GetStatus 获取服务状态
func (s *Service) GetStatus() *SyncStatus {
	var totalFiles int64
	var totalSize int64
	var uploading int64

	s.db.Model(&SyncFileModel{}).Where("status = ?", "completed").Count(&totalFiles)
	s.db.Model(&SyncFileModel{}).Where("status = ?", "completed").Select("COALESCE(SUM(size), 0)").Scan(&totalSize)
	s.db.Model(&UploadSessionModel{}).Count(&uploading)

	return &SyncStatus{
		Running:     true,
		StoragePath: s.filesDir,
		TotalFiles:  int(totalFiles),
		TotalSize:   totalSize,
		Uploading:   int(uploading),
	}
}

// ListFiles 获取文件列表
func (s *Service) ListFiles(req *ListFilesRequest) (*ListFilesResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	var total int64
	s.db.Model(&SyncFileModel{}).Where("status = ?", "completed").Count(&total)

	var models []SyncFileModel
	if err := s.db.Where("status = ?", "completed").
		Order("created_at DESC").
		Limit(limit).
		Offset(req.Offset).
		Find(&models).Error; err != nil {
		return nil, err
	}

	files := make([]SyncFile, len(models))
	for i, m := range models {
		files[i] = SyncFile{
			ID:        m.ID,
			Filename:  m.Filename,
			Size:      m.Size,
			MimeType:  m.MimeType,
			SHA256:    m.SHA256,
			Path:      m.Path,
			Status:    m.Status,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		}
	}

	return &ListFilesResponse{
		Files: files,
		Total: int(total),
	}, nil
}

// GetFile 获取单个文件
func (s *Service) GetFile(id string) (*SyncFile, error) {
	var m SyncFileModel
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &SyncFile{
		ID:        m.ID,
		Filename:  m.Filename,
		Size:      m.Size,
		MimeType:  m.MimeType,
		SHA256:    m.SHA256,
		Path:      m.Path,
		Status:    m.Status,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

// DeleteFile 删除文件
func (s *Service) DeleteFile(id string) error {
	var m SyncFileModel
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return err
	}

	// 删除物理文件
	os.Remove(m.Path)

	// 删除数据库记录
	return s.db.Delete(&m).Error
}

// ListActiveUploads 获取进行中的上传
func (s *Service) ListActiveUploads() ([]UploadSession, error) {
	var models []UploadSessionModel
	if err := s.db.Where("expires_at > ?", time.Now()).
		Order("created_at DESC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	sessions := make([]UploadSession, len(models))
	for i, m := range models {
		progress := float64(0)
		if m.Size > 0 {
			progress = float64(m.Offset) / float64(m.Size)
		}
		sessions[i] = UploadSession{
			ID:        m.ID,
			Filename:  m.Filename,
			Size:      m.Size,
			Offset:    m.Offset,
			Progress:  progress,
			CreatedAt: m.CreatedAt,
			ExpiresAt: m.ExpiresAt,
		}
	}

	return sessions, nil
}

// GetUpload 获取上传会话
func (s *Service) GetUpload(id string) (*UploadSession, error) {
	var m UploadSessionModel
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}

	progress := float64(0)
	if m.Size > 0 {
		progress = float64(m.Offset) / float64(m.Size)
	}

	return &UploadSession{
		ID:        m.ID,
		Filename:  m.Filename,
		Size:      m.Size,
		Offset:    m.Offset,
		Progress:  progress,
		CreatedAt: m.CreatedAt,
		ExpiresAt: m.ExpiresAt,
	}, nil
}

// DownloadFile 获取文件下载路径
func (s *Service) DownloadFile(id string) (string, string, error) {
	var m SyncFileModel
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return "", "", err
	}
	return m.Path, m.Filename, nil
}
