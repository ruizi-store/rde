package retrogame

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var removeAll = os.RemoveAll

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

// GetStatus 获取 EmulatorJS 安装状态
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.service.GetStatus()
	c.JSON(http.StatusOK, status)
}

// Setup 下载安装 EmulatorJS（SSE 流式返回进度）
func (h *Handler) Setup(c *gin.Context) {
	if h.service.IsInstalled() {
		c.JSON(http.StatusOK, gin.H{
			"status":  "completed",
			"message": "EmulatorJS 已安装",
		})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	progressChan := make(chan ProgressEvent, 20)

	go func() {
		h.service.Setup(progressChan)
	}()

	c.Stream(func(w io.Writer) bool {
		if event, ok := <-progressChan; ok {
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
			return true
		}
		fmt.Fprintf(w, "event: done\ndata: {\"status\":\"done\"}\n\n")
		return false
	})
}

// Uninstall 卸载 EmulatorJS
func (h *Handler) Uninstall(c *gin.Context) {
	if !h.service.IsInstalled() {
		c.JSON(http.StatusOK, gin.H{"message": "EmulatorJS 未安装"})
		return
	}

	emulatorDir := h.service.GetEmulatorDir()
	h.logger.Info("Uninstalling EmulatorJS", zap.String("dir", emulatorDir))

	if err := removeAll(emulatorDir); err != nil {
		h.logger.Error("Failed to uninstall EmulatorJS", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "卸载失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "EmulatorJS 已卸载"})
}

// ScanRoms 扫描指定目录中的 ROM 文件
// GET /retrogame/scan-roms?path=/home/user/Games/ROMs
func (h *Handler) ScanRoms(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 参数不能为空"})
		return
	}

	// 检查目录是否存在
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 目录不存在时返回空列表而非错误
			c.JSON(http.StatusOK, []RomFileInfo{})
			return
		}
		h.logger.Error("Failed to stat ROM directory", zap.String("path", path), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法访问目录"})
		return
	}
	if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "指定路径不是目录"})
		return
	}

	roms, err := h.service.ScanRoms(path)
	if err != nil {
		h.logger.Error("Failed to scan ROMs", zap.String("path", path), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描 ROM 失败"})
		return
	}

	c.JSON(http.StatusOK, roms)
}
