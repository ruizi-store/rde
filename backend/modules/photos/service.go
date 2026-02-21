// Package photos 照片管理模块
package photos

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ruizi-store/rde/backend/modules/files"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrLibraryNotFound = errors.New("library not found")
	ErrPhotoNotFound   = errors.New("photo not found")
	ErrAlbumNotFound   = errors.New("album not found")
	ErrPathNotExist    = errors.New("path does not exist")
	ErrPathNotDir      = errors.New("path is not a directory")
	ErrLibraryExists   = errors.New("library already exists")
)

// 支持的媒体文件扩展名
var (
	photoExtensions = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
		".heic": true, ".heif": true, ".raw": true, ".cr2": true,
		".nef": true, ".arw": true, ".dng": true, ".orf": true,
	}
	videoExtensions = map[string]bool{
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".3gp": true, ".mts": true, ".m2ts": true,
	}
)

// Service 照片服务
type Service struct {
	logger   *zap.Logger
	db       *gorm.DB
	thumbSvc *files.ThumbnailService
}

// NewService 创建照片服务
func NewService(logger *zap.Logger, db *gorm.DB, thumbSvc *files.ThumbnailService) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		logger:   logger,
		db:       db,
		thumbSvc: thumbSvc,
	}
}

// ============ 图库管理 ============

// CreateLibrary 创建图库
func (s *Service) CreateLibrary(userID string, req *CreateLibraryRequest) (*Library, error) {
	// 规范化路径
	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		return nil, err
	}

	// 验证路径，如果不存在则创建
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 目录不存在，自动创建
			if err := os.MkdirAll(absPath, 0755); err != nil {
				s.logger.Warn("Failed to create library directory",
					zap.String("path", absPath),
					zap.Error(err))
				return nil, ErrPathNotExist
			}
			s.logger.Info("Created library directory", zap.String("path", absPath))
		} else {
			// 其他错误
			return nil, err
		}
	} else if !info.IsDir() {
		// 路径存在但不是目录
		return nil, ErrPathNotDir
	}

	// 检查是否已存在
	var count int64
	s.db.Model(&Library{}).Where("path = ?", absPath).Count(&count)
	if count > 0 {
		return nil, ErrLibraryExists
	}

	library := &Library{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Path:        absPath,
		UserID:      userID,
		ScanEnabled: req.ScanEnabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(library).Error; err != nil {
		return nil, err
	}

	s.logger.Info("library created",
		zap.String("id", library.ID),
		zap.String("path", library.Path))

	return library, nil
}

// GetLibrary 获取图库
func (s *Service) GetLibrary(id string) (*Library, error) {
	var library Library
	if err := s.db.First(&library, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLibraryNotFound
		}
		return nil, err
	}
	return &library, nil
}

// ListLibraries 列出用户的图库
func (s *Service) ListLibraries(userID string) ([]Library, error) {
	var libraries []Library
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&libraries).Error; err != nil {
		return nil, err
	}
	return libraries, nil
}

// UpdateLibrary 更新图库
func (s *Service) UpdateLibrary(id string, req *UpdateLibraryRequest) (*Library, error) {
	library, err := s.GetLibrary(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		library.Name = *req.Name
	}
	if req.ScanEnabled != nil {
		library.ScanEnabled = *req.ScanEnabled
	}
	library.UpdatedAt = time.Now()

	if err := s.db.Save(library).Error; err != nil {
		return nil, err
	}
	return library, nil
}

// DeleteLibrary 删除图库
func (s *Service) DeleteLibrary(id string) error {
	library, err := s.GetLibrary(id)
	if err != nil {
		return err
	}

	// 删除关联的照片记录（不删除文件）
	if err := s.db.Where("library_id = ?", id).Delete(&Photo{}).Error; err != nil {
		return err
	}

	// 删除图库
	if err := s.db.Delete(library).Error; err != nil {
		return err
	}

	s.logger.Info("library deleted", zap.String("id", id))
	return nil
}

// ============ 照片管理 ============

// GetPhoto 获取照片
func (s *Service) GetPhoto(id string) (*Photo, error) {
	var photo Photo
	if err := s.db.Preload("Library").First(&photo, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPhotoNotFound
		}
		return nil, err
	}
	return &photo, nil
}

// GetThumbnail 获取照片缩略图
func (s *Service) GetThumbnail(photoPath string, size files.ThumbnailSize) (*files.ThumbnailResult, error) {
	if s.thumbSvc == nil {
		return nil, fmt.Errorf("thumbnail service not available")
	}
	return s.thumbSvc.GetThumbnail(photoPath, size)
}

// FindPhotoByPath 通过路径查找照片
func (s *Service) FindPhotoByPath(path string) (*Photo, error) {
	var photo Photo
	if err := s.db.Preload("Library").First(&photo, "path = ?", path).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 返回 nil 而不是错误
		}
		return nil, err
	}
	return &photo, nil
}

// ListPhotos 列出照片
func (s *Service) ListPhotos(req *ListPhotosRequest) (*ListPhotosResponse, error) {
	query := s.db.Model(&Photo{}).Where("is_deleted = ?", false)

	// 应用过滤条件
	if req.LibraryID != "" {
		query = query.Where("library_id = ?", req.LibraryID)
	}
	if req.Type == "photo" {
		query = query.Where("type = ?", "photo")
	} else if req.Type == "video" {
		query = query.Where("type = ?", "video")
	}
	if req.Favorite != nil && *req.Favorite {
		query = query.Where("is_favorite = ?", true)
	}
	if req.Archived != nil && *req.Archived {
		query = query.Where("is_archived = ?", true)
	}
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			query = query.Where("taken_at >= ?", t)
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			query = query.Where("taken_at <= ?", t.Add(24*time.Hour))
		}
	}

	// 计算总数
	var total int64
	query.Count(&total)

	// 排序
	switch req.Sort {
	case "date_asc":
		query = query.Order("taken_at ASC, created_at ASC")
	case "name_asc":
		query = query.Order("filename ASC")
	case "name_desc":
		query = query.Order("filename DESC")
	default: // date_desc
		query = query.Order("taken_at DESC, created_at DESC")
	}

	// 分页
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 200 {
		req.Limit = 200
	}
	query = query.Offset(req.Offset).Limit(req.Limit)

	var photos []Photo
	if err := query.Find(&photos).Error; err != nil {
		return nil, err
	}

	// 转换为响应
	response := &ListPhotosResponse{
		Photos: make([]PhotoResponse, len(photos)),
		Total:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
	}
	for i, p := range photos {
		response.Photos[i] = s.toPhotoResponse(&p)
	}

	return response, nil
}

// UpdatePhoto 更新照片
func (s *Service) UpdatePhoto(id string, req *UpdatePhotoRequest) (*Photo, error) {
	photo, err := s.GetPhoto(id)
	if err != nil {
		return nil, err
	}

	if req.IsFavorite != nil {
		photo.IsFavorite = *req.IsFavorite
	}
	if req.IsArchived != nil {
		photo.IsArchived = *req.IsArchived
	}
	if req.TakenAt != nil {
		photo.TakenAt = req.TakenAt
	}
	photo.UpdatedAt = time.Now()

	if err := s.db.Save(photo).Error; err != nil {
		return nil, err
	}
	return photo, nil
}

// DeletePhoto 删除照片（移入回收站）
func (s *Service) DeletePhoto(id string, force bool) error {
	photo, err := s.GetPhoto(id)
	if err != nil {
		return err
	}

	if force {
		// 永久删除
		return s.db.Delete(photo).Error
	}

	// 移入回收站
	now := time.Now()
	photo.IsDeleted = true
	photo.DeletedAt = &now
	photo.UpdatedAt = now
	return s.db.Save(photo).Error
}

// BatchDeletePhotos 批量删除照片
func (s *Service) BatchDeletePhotos(ids []string, force bool) error {
	if force {
		return s.db.Where("id IN ?", ids).Delete(&Photo{}).Error
	}

	now := time.Now()
	return s.db.Model(&Photo{}).Where("id IN ?", ids).Updates(map[string]interface{}{
		"is_deleted": true,
		"deleted_at": now,
		"updated_at": now,
	}).Error
}

// RestorePhoto 恢复照片
func (s *Service) RestorePhoto(id string) error {
	return s.db.Model(&Photo{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_deleted": false,
		"deleted_at": nil,
		"updated_at": time.Now(),
	}).Error
}

// EmptyTrash 清空回收站
func (s *Service) EmptyTrash(userID string) error {
	return s.db.Where("is_deleted = ? AND library_id IN (SELECT id FROM libraries WHERE user_id = ?)", true, userID).
		Delete(&Photo{}).Error
}

// ============ 相册管理 ============

// CreateAlbum 创建相册
func (s *Service) CreateAlbum(userID string, req *CreateAlbumRequest) (*Album, error) {
	album := &Album{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Type:        "manual",
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(album).Error; err != nil {
		return nil, err
	}

	// 添加初始照片
	if len(req.PhotoIDs) > 0 {
		if err := s.addPhotosToAlbum(album.ID, req.PhotoIDs); err != nil {
			s.logger.Warn("failed to add initial photos to album", zap.Error(err))
		}
	}

	return album, nil
}

// GetAlbum 获取相册
func (s *Service) GetAlbum(id string) (*Album, error) {
	var album Album
	if err := s.db.First(&album, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAlbumNotFound
		}
		return nil, err
	}
	return &album, nil
}

// ListAlbums 列出相册
func (s *Service) ListAlbums(userID string) ([]AlbumResponse, error) {
	var albums []Album
	if err := s.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&albums).Error; err != nil {
		return nil, err
	}

	responses := make([]AlbumResponse, len(albums))
	for i, album := range albums {
		responses[i] = AlbumResponse{
			Album:    album,
			CoverURL: s.getAlbumCoverURL(&album),
		}
	}
	return responses, nil
}

// UpdateAlbum 更新相册
func (s *Service) UpdateAlbum(id string, req *UpdateAlbumRequest) (*Album, error) {
	album, err := s.GetAlbum(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		album.Name = *req.Name
	}
	if req.Description != nil {
		album.Description = *req.Description
	}
	if req.CoverID != nil {
		album.CoverID = *req.CoverID
	}
	if req.SortOrder != nil {
		album.SortOrder = *req.SortOrder
	}
	album.UpdatedAt = time.Now()

	if err := s.db.Save(album).Error; err != nil {
		return nil, err
	}
	return album, nil
}

// DeleteAlbum 删除相册
func (s *Service) DeleteAlbum(id string) error {
	// 删除关联
	if err := s.db.Where("album_id = ?", id).Delete(&AlbumPhoto{}).Error; err != nil {
		return err
	}
	return s.db.Delete(&Album{}, "id = ?", id).Error
}

// AddPhotosToAlbum 添加照片到相册
func (s *Service) AddPhotosToAlbum(albumID string, photoIDs []string) error {
	if _, err := s.GetAlbum(albumID); err != nil {
		return err
	}
	return s.addPhotosToAlbum(albumID, photoIDs)
}

func (s *Service) addPhotosToAlbum(albumID string, photoIDs []string) error {
	now := time.Now()
	for i, photoID := range photoIDs {
		ap := &AlbumPhoto{
			AlbumID:   albumID,
			PhotoID:   photoID,
			SortOrder: i,
			AddedAt:   now,
		}
		// 忽略重复
		s.db.Clauses().FirstOrCreate(ap, "album_id = ? AND photo_id = ?", albumID, photoID)
	}

	// 更新照片数量
	var count int64
	s.db.Model(&AlbumPhoto{}).Where("album_id = ?", albumID).Count(&count)
	s.db.Model(&Album{}).Where("id = ?", albumID).Update("photo_count", count)

	return nil
}

// RemovePhotoFromAlbum 从相册移除照片
func (s *Service) RemovePhotoFromAlbum(albumID, photoID string) error {
	if err := s.db.Where("album_id = ? AND photo_id = ?", albumID, photoID).Delete(&AlbumPhoto{}).Error; err != nil {
		return err
	}

	// 更新照片数量
	var count int64
	s.db.Model(&AlbumPhoto{}).Where("album_id = ?", albumID).Count(&count)
	s.db.Model(&Album{}).Where("id = ?", albumID).Update("photo_count", count)

	return nil
}

// GetAlbumPhotos 获取相册照片
func (s *Service) GetAlbumPhotos(albumID string, offset, limit int) (*ListPhotosResponse, error) {
	if limit <= 0 {
		limit = 50
	}

	var total int64
	s.db.Model(&AlbumPhoto{}).Where("album_id = ?", albumID).Count(&total)

	var albumPhotos []AlbumPhoto
	if err := s.db.Where("album_id = ?", albumID).Order("sort_order ASC").
		Offset(offset).Limit(limit).Find(&albumPhotos).Error; err != nil {
		return nil, err
	}

	photoIDs := make([]string, len(albumPhotos))
	for i, ap := range albumPhotos {
		photoIDs[i] = ap.PhotoID
	}

	var photos []Photo
	if len(photoIDs) > 0 {
		s.db.Where("id IN ?", photoIDs).Find(&photos)
	}

	// 保持顺序
	photoMap := make(map[string]*Photo)
	for i := range photos {
		photoMap[photos[i].ID] = &photos[i]
	}

	response := &ListPhotosResponse{
		Photos: make([]PhotoResponse, 0, len(albumPhotos)),
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}
	for _, ap := range albumPhotos {
		if p, ok := photoMap[ap.PhotoID]; ok {
			response.Photos = append(response.Photos, s.toPhotoResponse(p))
		}
	}

	return response, nil
}

// ============ 时间线 ============

// GetTimeline 获取时间线
func (s *Service) GetTimeline(req *TimelineRequest) (*TimelineResponse, error) {
	query := s.db.Model(&Photo{}).Where("is_deleted = ?", false)

	if req.LibraryID != "" {
		query = query.Where("library_id = ?", req.LibraryID)
	}
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			query = query.Where("COALESCE(taken_at, created_at) >= ?", t)
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			query = query.Where("COALESCE(taken_at, created_at) <= ?", t.Add(24*time.Hour))
		}
	}

	// 按日期分组
	groupBy := req.GroupBy
	if groupBy == "" {
		groupBy = "day"
	}

	var dateFormat string
	switch groupBy {
	case "month":
		dateFormat = "%Y-%m"
	case "year":
		dateFormat = "%Y"
	default:
		dateFormat = "%Y-%m-%d"
	}

	type DateCount struct {
		Date  string
		Count int
	}

	var dateCounts []DateCount
	if err := query.Select("strftime(?, COALESCE(taken_at, created_at)) as date, COUNT(*) as count", dateFormat).
		Group("date").Order("date DESC").Find(&dateCounts).Error; err != nil {
		return nil, err
	}

	// 获取每个分组的照片
	groups := make([]TimelineGroup, 0, len(dateCounts))
	var total int64

	for _, dc := range dateCounts {
		total += int64(dc.Count)

		// 获取该日期的照片（只取前几张作为预览）
		var photos []Photo
		photoQuery := s.db.Model(&Photo{}).Where("is_deleted = ? AND strftime(?, COALESCE(taken_at, created_at)) = ?", false, dateFormat, dc.Date)
		if req.LibraryID != "" {
			photoQuery = photoQuery.Where("library_id = ?", req.LibraryID)
		}
		photoQuery.Order("COALESCE(taken_at, created_at) DESC").Limit(200).Find(&photos)

		photoResponses := make([]PhotoResponse, len(photos))
		for i, p := range photos {
			photoResponses[i] = s.toPhotoResponse(&p)
		}

		groups = append(groups, TimelineGroup{
			Date:   dc.Date,
			Count:  dc.Count,
			Photos: photoResponses,
		})
	}

	return &TimelineResponse{
		Groups: groups,
		Total:  total,
	}, nil
}

// GetCalendar 获取日历数据
func (s *Service) GetCalendar(libraryID string, year, month int) (*CalendarResponse, error) {
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)

	query := s.db.Model(&Photo{}).Where("is_deleted = ? AND taken_at >= ? AND taken_at < ?", false, startDate, endDate)
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}

	type DateCount struct {
		Date  string
		Count int
	}

	var dateCounts []DateCount
	if err := query.Select("strftime('%Y-%m-%d', taken_at) as date, COUNT(*) as count").
		Group("date").Find(&dateCounts).Error; err != nil {
		return nil, err
	}

	days := make([]CalendarDay, len(dateCounts))
	for i, dc := range dateCounts {
		days[i] = CalendarDay{
			Date:  dc.Date,
			Count: dc.Count,
		}
	}

	return &CalendarResponse{Days: days}, nil
}

// ============ 统计 ============

// GetStats 获取统计信息
func (s *Service) GetStats(userID string) (*StatsResponse, error) {
	stats := &StatsResponse{}

	// 获取用户的图库 ID
	var libraryIDs []string
	s.db.Model(&Library{}).Where("user_id = ?", userID).Pluck("id", &libraryIDs)

	if len(libraryIDs) > 0 {
		s.db.Model(&Photo{}).Where("library_id IN ? AND type = ? AND is_deleted = ?", libraryIDs, "photo", false).Count(&stats.TotalPhotos)
		s.db.Model(&Photo{}).Where("library_id IN ? AND type = ? AND is_deleted = ?", libraryIDs, "video", false).Count(&stats.TotalVideos)
		s.db.Model(&Photo{}).Where("library_id IN ? AND is_favorite = ? AND is_deleted = ?", libraryIDs, true, false).Count(&stats.FavoriteCount)
		s.db.Model(&Photo{}).Where("library_id IN ? AND is_archived = ? AND is_deleted = ?", libraryIDs, true, false).Count(&stats.ArchivedCount)
		s.db.Model(&Photo{}).Where("library_id IN ? AND is_deleted = ?", libraryIDs, true).Count(&stats.TrashCount)

		var totalSize int64
		s.db.Model(&Photo{}).Where("library_id IN ? AND is_deleted = ?", libraryIDs, false).Select("COALESCE(SUM(size), 0)").Scan(&totalSize)
		stats.TotalSize = totalSize
	}

	s.db.Model(&Album{}).Where("user_id = ?", userID).Count(&stats.TotalAlbums)

	return stats, nil
}

// ============ 辅助方法 ============

// toPhotoResponse 转换为响应
func (s *Service) toPhotoResponse(p *Photo) PhotoResponse {
	return PhotoResponse{
		Photo:        *p,
		ThumbnailURL: fmt.Sprintf("/api/v1/photos/%s/thumbnail", p.ID),
		PreviewURL:   fmt.Sprintf("/api/v1/photos/%s/preview", p.ID),
		OriginalURL:  fmt.Sprintf("/api/v1/photos/%s/original", p.ID),
	}
}

// getAlbumCoverURL 获取相册封面 URL
func (s *Service) getAlbumCoverURL(album *Album) string {
	if album.CoverID != "" {
		return fmt.Sprintf("/api/v1/photos/%s/thumbnail", album.CoverID)
	}

	// 获取第一张照片作为封面
	var ap AlbumPhoto
	if err := s.db.Where("album_id = ?", album.ID).Order("sort_order ASC").First(&ap).Error; err == nil {
		return fmt.Sprintf("/api/v1/photos/%s/thumbnail", ap.PhotoID)
	}
	return ""
}

// IsMediaFile 检查是否为媒体文件
func IsMediaFile(path string) (bool, string) {
	ext := strings.ToLower(filepath.Ext(path))
	if photoExtensions[ext] {
		return true, "photo"
	}
	if videoExtensions[ext] {
		return true, "video"
	}
	return false, ""
}

// CalculateFileHash 计算文件哈希
func CalculateFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	// 使用固定大小缓冲区提高大文件性能
	buf := make([]byte, 1024*1024) // 1MB buffer
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetExistingPaths 批量查询已存在的路径
func (s *Service) GetExistingPaths(paths []string) (map[string]bool, error) {
	if len(paths) == 0 {
		return make(map[string]bool), nil
	}

	// 分批查询避免 SQL 过长
	const batchSize = 500
	existing := make(map[string]bool)

	for i := 0; i < len(paths); i += batchSize {
		end := i + batchSize
		if end > len(paths) {
			end = len(paths)
		}
		batch := paths[i:end]

		var foundPaths []string
		if err := s.db.Model(&Photo{}).
			Where("path IN ?", batch).
			Pluck("path", &foundPaths).Error; err != nil {
			return nil, err
		}

		for _, p := range foundPaths {
			existing[p] = true
		}
	}

	return existing, nil
}
