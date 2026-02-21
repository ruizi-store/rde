package video

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/video")
	{
		g.GET("/stream", h.handleStream)
		g.GET("/info", h.handleInfo)
		g.GET("/thumbnail", h.handleThumbnail)
		g.GET("/subtitles", h.handleSubtitles)
		g.GET("/subtitle", h.handleSubtitle)
		g.POST("/hls/start", h.handleStartHLS)
		g.GET("/hls/playlist/:session", h.handleHLSPlaylist)
		g.GET("/hls/segment/:session/:segment", h.handleHLSSegment)
		g.DELETE("/hls/:session", h.handleStopHLS)
	}
}

// handleStream 直接流式传输视频
func (h *Handler) handleStream(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	// 检查文件是否存在
	stat, err := os.Stat(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	file, err := os.Open(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open file failed"})
		return
	}
	defer file.Close()

	// 获取 MIME 类型
	ext := strings.ToLower(filepath.Ext(path))
	mimeType := VideoMimeTypes[ext]
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	// 处理 Range 请求
	fileSize := stat.Size()
	rangeHeader := c.GetHeader("Range")

	if rangeHeader == "" {
		// 完整文件请求
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
		c.Header("Content-Type", mimeType)
		c.Header("Accept-Ranges", "bytes")
		c.Status(http.StatusOK)
		io.Copy(c.Writer, file)
		return
	}

	// 解析 Range
	ranges := strings.Replace(rangeHeader, "bytes=", "", 1)
	parts := strings.Split(ranges, "-")

	start, _ := strconv.ParseInt(parts[0], 10, 64)
	end := fileSize - 1
	if len(parts) > 1 && parts[1] != "" {
		end, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	if start > end || start < 0 || end >= fileSize {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1
	file.Seek(start, 0)

	c.Header("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(fileSize, 10))
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Type", mimeType)
	c.Header("Accept-Ranges", "bytes")
	c.Status(http.StatusPartialContent)

	io.CopyN(c.Writer, file, contentLength)
}

// handleInfo 获取视频信息
func (h *Handler) handleInfo(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	info, err := h.service.GetVideoInfo(path)
	if err != nil {
		h.logger.Error("get video info failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// handleThumbnail 获取视频缩略图
func (h *Handler) handleThumbnail(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	timestamp, _ := strconv.ParseFloat(c.Query("t"), 64)

	thumbPath, err := h.service.GenerateThumbnail(path, timestamp)
	if err != nil {
		h.logger.Error("generate thumbnail failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.File(thumbPath)
}

// handleSubtitles 获取可用字幕列表
func (h *Handler) handleSubtitles(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	result, err := h.service.GetSubtitles(path)
	if err != nil {
		h.logger.Error("get subtitles failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleSubtitle 获取字幕内容（转换为 VTT）
func (h *Handler) handleSubtitle(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	embedded := c.Query("embedded") == "true"
	index, _ := strconv.Atoi(c.Query("index"))

	var data []byte
	var err error

	if embedded {
		data, err = h.service.ExtractEmbeddedSubtitle(path, index)
	} else {
		data, err = h.service.ConvertSubtitleToVTT(path)
	}

	if err != nil {
		h.logger.Error("get subtitle failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Data(http.StatusOK, "text/vtt", data)
}

// handleStartHLS 启动 HLS 转码
func (h *Handler) handleStartHLS(c *gin.Context) {
	var req struct {
		Path      string  `json:"path"`
		StartTime float64 `json:"startTime"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	sessionID, err := h.service.StartHLSTranscode(req.Path, req.StartTime)
	if err != nil {
		h.logger.Error("start HLS transcode failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessionId": sessionID})
}

// handleHLSPlaylist 获取 HLS 播放列表
func (h *Handler) handleHLSPlaylist(c *gin.Context) {
	session := c.Param("session")

	playlistPath, err := h.service.GetHLSPlaylist(session)
	if err != nil {
		h.logger.Error("get HLS playlist failed", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.File(playlistPath)
}

// handleHLSSegment 获取 HLS 分片
func (h *Handler) handleHLSSegment(c *gin.Context) {
	session := c.Param("session")
	segment := c.Param("segment")

	segmentPath, err := h.service.GetHLSSegment(session, segment)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.File(segmentPath)
}

// handleStopHLS 停止 HLS 转码
func (h *Handler) handleStopHLS(c *gin.Context) {
	session := c.Param("session")
	h.service.StopTranscode(session)
	c.Status(http.StatusNoContent)
}
