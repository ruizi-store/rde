// Package photos 照片管理模块
package photos

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/modules/files"
)

// Handler HTTP 处理器
type Handler struct {
	service   *Service
	scanner   *Scanner
	aiService *AIService
}

// NewHandler 创建处理器
func NewHandler(service *Service, scanner *Scanner, config Config) *Handler {
	return &Handler{
		service:   service,
		scanner:   scanner,
		aiService: NewAIService(config.EnableAI, config.IndexerURL),
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	photos := r.Group("/photos")
	{
		// 图库管理
		photos.POST("/libraries", h.CreateLibrary)
		photos.GET("/libraries", h.ListLibraries)
		photos.GET("/libraries/:id", h.GetLibrary)
		photos.PUT("/libraries/:id", h.UpdateLibrary)
		photos.DELETE("/libraries/:id", h.DeleteLibrary)
		photos.POST("/libraries/:id/scan", h.ScanLibrary)
		photos.GET("/libraries/:id/progress", h.GetScanProgress)

		// 照片
		photos.GET("", h.ListPhotos)
		photos.GET("/:id", h.GetPhoto)
		photos.PUT("/:id", h.UpdatePhoto)
		photos.DELETE("/:id", h.DeletePhoto)
		photos.GET("/:id/thumbnail", h.GetThumbnail)
		photos.GET("/:id/preview", h.GetPreview)
		photos.GET("/:id/original", h.GetOriginal)

		// 批量操作
		photos.POST("/batch/delete", h.BatchDelete)
		photos.POST("/batch/favorite", h.BatchFavorite)
		photos.POST("/batch/archive", h.BatchArchive)

		// 相册
		photos.POST("/albums", h.CreateAlbum)
		photos.GET("/albums", h.ListAlbums)
		photos.GET("/albums/:id", h.GetAlbum)
		photos.PUT("/albums/:id", h.UpdateAlbum)
		photos.DELETE("/albums/:id", h.DeleteAlbum)
		photos.GET("/albums/:id/photos", h.GetAlbumPhotos)
		photos.POST("/albums/:id/photos", h.AddPhotosToAlbum)
		photos.DELETE("/albums/:id/photos/:photoId", h.RemovePhotoFromAlbum)

		// 时间线
		photos.GET("/timeline", h.GetTimeline)
		photos.GET("/calendar", h.GetCalendar)

		// 回收站
		photos.GET("/trash", h.ListTrash)
		photos.POST("/trash/:id/restore", h.RestorePhoto)
		photos.DELETE("/trash", h.EmptyTrash)

		// AI 智能搜索
		photos.GET("/ai/status", h.GetAIStatus)
		photos.GET("/ai/search", h.AISearch)
		photos.GET("/ai/search/face", h.AISearchFace)
		photos.GET("/ai/search/text", h.AISearchText)
		photos.POST("/ai/index", h.TriggerAIIndex)

		// 统计
		photos.GET("/stats", h.GetStats)
	}
}

// getUserID 从上下文获取用户ID
func getUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

// ============ 图库管理 ============

// CreateLibrary 创建图库
func (h *Handler) CreateLibrary(c *gin.Context) {
	var req CreateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)
	library, err := h.service.CreateLibrary(userID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case ErrPathNotExist, ErrPathNotDir:
			status = http.StatusBadRequest
		case ErrLibraryExists:
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// 异步触发扫描
	go h.scanner.ScanLibrary(context.Background(), library.ID)

	c.JSON(http.StatusCreated, library)
}

// ListLibraries 列出图库
func (h *Handler) ListLibraries(c *gin.Context) {
	userID := getUserID(c)
	libraries, err := h.service.ListLibraries(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 添加扫描状态
	responses := make([]LibraryResponse, len(libraries))
	for i, lib := range libraries {
		responses[i] = LibraryResponse{
			Library:  lib,
			Scanning: h.scanner.IsScanning(lib.ID),
		}
	}

	c.JSON(http.StatusOK, responses)
}

// GetLibrary 获取图库
func (h *Handler) GetLibrary(c *gin.Context) {
	library, err := h.service.GetLibrary(c.Param("id"))
	if err != nil {
		if err == ErrLibraryNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, LibraryResponse{
		Library:  *library,
		Scanning: h.scanner.IsScanning(library.ID),
	})
}

// UpdateLibrary 更新图库
func (h *Handler) UpdateLibrary(c *gin.Context) {
	var req UpdateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	library, err := h.service.UpdateLibrary(c.Param("id"), &req)
	if err != nil {
		if err == ErrLibraryNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, library)
}

// DeleteLibrary 删除图库
func (h *Handler) DeleteLibrary(c *gin.Context) {
	if err := h.service.DeleteLibrary(c.Param("id")); err != nil {
		if err == ErrLibraryNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ScanLibrary 触发扫描
func (h *Handler) ScanLibrary(c *gin.Context) {
	libraryID := c.Param("id")

	if h.scanner.IsScanning(libraryID) {
		c.JSON(http.StatusConflict, gin.H{"error": "scan already in progress"})
		return
	}

	go h.scanner.ScanLibrary(context.Background(), libraryID)

	c.JSON(http.StatusAccepted, gin.H{"status": "scan started"})
}

// GetScanProgress 获取扫描进度
func (h *Handler) GetScanProgress(c *gin.Context) {
	progress := h.scanner.GetProgress(c.Param("id"))
	c.JSON(http.StatusOK, progress)
}

// ============ 照片 ============

// ListPhotos 列出照片
func (h *Handler) ListPhotos(c *gin.Context) {
	var req ListPhotosRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.ListPhotos(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetPhoto 获取照片
func (h *Handler) GetPhoto(c *gin.Context) {
	photo, err := h.service.GetPhoto(c.Param("id"))
	if err != nil {
		if err == ErrPhotoNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.service.toPhotoResponse(photo))
}

// UpdatePhoto 更新照片
func (h *Handler) UpdatePhoto(c *gin.Context) {
	var req UpdatePhotoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	photo, err := h.service.UpdatePhoto(c.Param("id"), &req)
	if err != nil {
		if err == ErrPhotoNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// DeletePhoto 删除照片
func (h *Handler) DeletePhoto(c *gin.Context) {
	force := c.Query("force") == "true"
	if err := h.service.DeletePhoto(c.Param("id"), force); err != nil {
		if err == ErrPhotoNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetThumbnail 获取缩略图
func (h *Handler) GetThumbnail(c *gin.Context) {
	h.serveImage(c, files.ThumbnailMedium)
}

// GetPreview 获取预览图
func (h *Handler) GetPreview(c *gin.Context) {
	h.serveImage(c, files.ThumbnailXLarge)
}

// GetOriginal 获取原图
func (h *Handler) GetOriginal(c *gin.Context) {
	photo, err := h.service.GetPhoto(c.Param("id"))
	if err != nil {
		if err == ErrPhotoNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.File(photo.Path)
}

// serveImage 提供图片
func (h *Handler) serveImage(c *gin.Context, size files.ThumbnailSize) {
	photo, err := h.service.GetPhoto(c.Param("id"))
	if err != nil {
		if err == ErrPhotoNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 使用缩略图服务生成/获取缩略图
	result, err := h.service.GetThumbnail(photo.Path, size)
	if err != nil {
		// 缩略图生成失败，返回原图
		c.File(photo.Path)
		return
	}

	// 设置缓存头
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("Content-Type", result.MimeType)

	// 如果有生成的缩略图文件，使用文件路径提供
	if result.Path != "" {
		c.File(result.Path)
		return
	}

	// 否则直接返回数据
	c.Data(http.StatusOK, result.MimeType, result.Data)
}

// ============ 批量操作 ============

// BatchDelete 批量删除
func (h *Handler) BatchDelete(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.BatchDeletePhotos(req.PhotoIDs, req.Force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": len(req.PhotoIDs)})
}

// BatchFavorite 批量收藏
func (h *Handler) BatchFavorite(c *gin.Context) {
	var req struct {
		PhotoIDs   []string `json:"photo_ids" binding:"required"`
		IsFavorite bool     `json:"is_favorite"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, id := range req.PhotoIDs {
		h.service.UpdatePhoto(id, &UpdatePhotoRequest{IsFavorite: &req.IsFavorite})
	}

	c.JSON(http.StatusOK, gin.H{"updated": len(req.PhotoIDs)})
}

// BatchArchive 批量归档
func (h *Handler) BatchArchive(c *gin.Context) {
	var req struct {
		PhotoIDs   []string `json:"photo_ids" binding:"required"`
		IsArchived bool     `json:"is_archived"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, id := range req.PhotoIDs {
		h.service.UpdatePhoto(id, &UpdatePhotoRequest{IsArchived: &req.IsArchived})
	}

	c.JSON(http.StatusOK, gin.H{"updated": len(req.PhotoIDs)})
}

// ============ 相册 ============

// CreateAlbum 创建相册
func (h *Handler) CreateAlbum(c *gin.Context) {
	var req CreateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)
	album, err := h.service.CreateAlbum(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, album)
}

// ListAlbums 列出相册
func (h *Handler) ListAlbums(c *gin.Context) {
	userID := getUserID(c)
	albums, err := h.service.ListAlbums(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, albums)
}

// GetAlbum 获取相册
func (h *Handler) GetAlbum(c *gin.Context) {
	album, err := h.service.GetAlbum(c.Param("id"))
	if err != nil {
		if err == ErrAlbumNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, album)
}

// UpdateAlbum 更新相册
func (h *Handler) UpdateAlbum(c *gin.Context) {
	var req UpdateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	album, err := h.service.UpdateAlbum(c.Param("id"), &req)
	if err != nil {
		if err == ErrAlbumNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, album)
}

// DeleteAlbum 删除相册
func (h *Handler) DeleteAlbum(c *gin.Context) {
	if err := h.service.DeleteAlbum(c.Param("id")); err != nil {
		if err == ErrAlbumNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetAlbumPhotos 获取相册照片
func (h *Handler) GetAlbumPhotos(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	response, err := h.service.GetAlbumPhotos(c.Param("id"), offset, limit)
	if err != nil {
		if err == ErrAlbumNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// AddPhotosToAlbum 添加照片到相册
func (h *Handler) AddPhotosToAlbum(c *gin.Context) {
	var req AddPhotosToAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.AddPhotosToAlbum(c.Param("id"), req.PhotoIDs); err != nil {
		if err == ErrAlbumNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"added": len(req.PhotoIDs)})
}

// RemovePhotoFromAlbum 从相册移除照片
func (h *Handler) RemovePhotoFromAlbum(c *gin.Context) {
	if err := h.service.RemovePhotoFromAlbum(c.Param("id"), c.Param("photoId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ============ 时间线 ============

// GetTimeline 获取时间线
func (h *Handler) GetTimeline(c *gin.Context) {
	var req TimelineRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.GetTimeline(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCalendar 获取日历数据
func (h *Handler) GetCalendar(c *gin.Context) {
	libraryID := c.Query("library_id")
	year, _ := strconv.Atoi(c.DefaultQuery("year", "2024"))
	month, _ := strconv.Atoi(c.DefaultQuery("month", "1"))

	response, err := h.service.GetCalendar(libraryID, year, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ============ 回收站 ============

// ListTrash 列出回收站
func (h *Handler) ListTrash(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	req := &ListPhotosRequest{
		Offset: offset,
		Limit:  limit,
	}

	// 查询已删除的照片
	var photos []Photo
	query := h.service.db.Model(&Photo{}).Where("is_deleted = ?", true).
		Order("deleted_at DESC").
		Offset(offset).Limit(limit)

	var total int64
	query.Count(&total)
	query.Find(&photos)

	response := &ListPhotosResponse{
		Photos: make([]PhotoResponse, len(photos)),
		Total:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
	}
	for i, p := range photos {
		response.Photos[i] = h.service.toPhotoResponse(&p)
	}

	c.JSON(http.StatusOK, response)
}

// RestorePhoto 恢复照片
func (h *Handler) RestorePhoto(c *gin.Context) {
	if err := h.service.RestorePhoto(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// EmptyTrash 清空回收站
func (h *Handler) EmptyTrash(c *gin.Context) {
	userID := getUserID(c)
	if err := h.service.EmptyTrash(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ============ 统计 ============

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	userID := getUserID(c)
	stats, err := h.service.GetStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ============ AI 智能搜索 ============

// GetAIStatus 获取 AI 服务状态
func (h *Handler) GetAIStatus(c *gin.Context) {
	if !h.aiService.IsEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"message": "AI indexer service is not enabled",
		})
		return
	}

	userID := getUserID(c)
	status, err := h.aiService.GetIndexerStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"enabled": true,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"status":  status,
	})
}

// AISearch 语义搜索
func (h *Handler) AISearch(c *gin.Context) {
	if !h.aiService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service not enabled"})
		return
	}

	userID := getUserID(c)
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	results, err := h.aiService.SemanticSearch(userID, query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为 Photo 对象
	photos, err := h.aiService.ConvertToPhotos(results, h.service)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  results.Total,
		"photos": photos,
	})
}

// AISearchFace 人脸属性搜索
func (h *Handler) AISearchFace(c *gin.Context) {
	if !h.aiService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service not enabled"})
		return
	}

	userID := getUserID(c)

	params := FaceSearchParams{}
	
	if ageMin := c.Query("age_min"); ageMin != "" {
		if v, err := strconv.Atoi(ageMin); err == nil {
			params.AgeMin = &v
		}
	}
	if ageMax := c.Query("age_max"); ageMax != "" {
		if v, err := strconv.Atoi(ageMax); err == nil {
			params.AgeMax = &v
		}
	}
	if gender := c.Query("gender"); gender == "male" || gender == "female" {
		params.Gender = gender
	}
	
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	params.Limit = limit

	results, err := h.aiService.SearchByFace(userID, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	photos, err := h.aiService.ConvertToPhotos(results, h.service)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  results.Total,
		"photos": photos,
	})
}

// AISearchText OCR 文字搜索
func (h *Handler) AISearchText(c *gin.Context) {
	if !h.aiService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service not enabled"})
		return
	}

	userID := getUserID(c)
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	results, err := h.aiService.SearchByText(userID, query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	photos, err := h.aiService.ConvertToPhotos(results, h.service)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  results.Total,
		"photos": photos,
	})
}

// TriggerAIIndex 触发 AI 索引
func (h *Handler) TriggerAIIndex(c *gin.Context) {
	if !h.aiService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service not enabled"})
		return
	}

	userID := getUserID(c)
	directory := c.Query("dir")

	if err := h.aiService.TriggerIndexing(userID, directory); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Indexing triggered"})
}
