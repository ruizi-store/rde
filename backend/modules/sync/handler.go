package sync

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	sync := r.Group("/sync")
	{
		// TUS 上传端点 - 使用统一处理器
		sync.Any("/upload", h.handleTus)
		sync.Any("/upload/*path", h.handleTus)

		// 同步管理 API
		sync.GET("/status", h.GetStatus)
		sync.GET("/files", h.ListFiles)
		sync.GET("/files/:id", h.GetFile)
		sync.GET("/files/:id/download", h.DownloadFile)
		sync.DELETE("/files/:id", h.DeleteFile)
		sync.GET("/uploads", h.ListUploads)
		sync.GET("/uploads/:id", h.GetUpload)
	}
}

// ==================== TUS 端点 ====================

func (h *Handler) setTusHeaders(c *gin.Context) {
	c.Header("Tus-Resumable", "1.0.0")
	c.Header("Tus-Version", "1.0.0")
	c.Header("Tus-Extension", "creation,creation-with-upload,termination")
	c.Header("Tus-Max-Size", "53687091200") // 50GB
}

func (h *Handler) handleTus(c *gin.Context) {
	h.setTusHeaders(c)
	
	// tusd 的 Handler 使用 strings.Trim(r.URL.Path, "/") 来路由：
	// - path = "" → POST 创建上传
	// - path = "{id}" → HEAD/PATCH/DELETE 操作上传
	// 需要把 /api/v1/sync/upload{/id} 转换为 {/id}
	
	path := c.Param("path") // 从 /*path 获取，如 "/abc123" 或空
	if path == "" {
		// 匹配 /upload，这是创建端点
		c.Request.URL.Path = "/"
	} else {
		// 匹配 /upload/*path，path 已经是 "/{id}" 格式
		c.Request.URL.Path = path
	}
	
	// 也可能直接请求 /upload/ (带尾部斜线)，这也是创建端点
	if strings.HasSuffix(c.Request.URL.Path, "/") && len(c.Request.URL.Path) > 1 {
		c.Request.URL.Path = "/"
	}
	
	handler := h.service.GetTusHandler()
	handler.ServeHTTP(c.Writer, c.Request)
}

// ==================== 同步管理 API ====================

// GetStatus 获取服务状态
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.service.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// ListFiles 获取文件列表
func (h *Handler) ListFiles(c *gin.Context) {
	var req ListFilesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	resp, err := h.service.ListFiles(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp.Files,
		"total":   resp.Total,
	})
}

// GetFile 获取文件详情
func (h *Handler) GetFile(c *gin.Context) {
	id := c.Param("id")
	file, err := h.service.GetFile(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "文件不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    file,
	})
}

// DownloadFile 下载文件
func (h *Handler) DownloadFile(c *gin.Context) {
	id := c.Param("id")
	path, filename, err := h.service.DownloadFile(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "文件不存在"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.File(path)
}

// DeleteFile 删除文件
func (h *Handler) DeleteFile(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteFile(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "文件不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListUploads 获取进行中的上传
func (h *Handler) ListUploads(c *gin.Context) {
	uploads, err := h.service.ListActiveUploads()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    uploads,
	})
}

// GetUpload 获取上传会话详情
func (h *Handler) GetUpload(c *gin.Context) {
	id := c.Param("id")
	upload, err := h.service.GetUpload(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "上传会话不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    upload,
	})
}
