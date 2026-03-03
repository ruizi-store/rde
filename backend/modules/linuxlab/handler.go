package linuxlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

// GetStatus 获取环境状态
// GET /linuxlab/status
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.service.GetStatus()
	c.JSON(http.StatusOK, status)
}

// Setup 初始化环境（拉取镜像+创建容器），SSE 流式输出
// POST /linuxlab/setup
func (h *Handler) Setup(c *gin.Context) {
	if h.service.ContainerRunning() {
		c.JSON(http.StatusOK, gin.H{"status": "completed", "message": "Linux Lab 容器已运行"})
		return
	}

	if h.service.IsSetting() {
		c.JSON(http.StatusConflict, gin.H{"error": "安装正在进行中"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	progressChan := make(chan ProgressEvent, 50)

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

// ListBoards 列出所有开发板
// GET /linuxlab/boards
func (h *Handler) ListBoards(c *gin.Context) {
	boards, err := h.service.ListBoards()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, boards)
}

// GetBoardDetail 获取指定开发板详情
// GET /linuxlab/boards/:arch/:mach
func (h *Handler) GetBoardDetail(c *gin.Context) {
	arch := c.Param("arch")
	mach := c.Param("mach")
	boardPath := arch + "/" + mach

	board, err := h.service.GetBoardDetail(boardPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, board)
}

// SwitchBoard 切换当前开发板
// POST /linuxlab/boards/switch
func (h *Handler) SwitchBoard(c *gin.Context) {
	var req SwitchBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.service.SwitchBoard(req.Board); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "切换成功", "board": req.Board})
}

// Build 触发构建（SSE 流式输出）
// POST /linuxlab/build
func (h *Handler) Build(c *gin.Context) {
	var req BuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if h.service.IsBuilding() {
		c.JSON(http.StatusConflict, gin.H{"error": "已有构建任务正在运行"})
		return
	}

	validTargets := map[string]bool{
		"kernel": true, "kernel-build": true,
		"uboot": true, "uboot-build": true,
		"root": true, "root-build": true, "root-rebuild": true,
		"modules": true, "modules-install": true,
		"all": true,
	}
	target := req.Target
	if target == "" {
		target = "kernel-build"
	}
	if !validTargets[target] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的构建目标: %s", target)})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	progressChan := make(chan ProgressEvent, 100)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		h.service.ExecMake(ctx, target, req.Board, progressChan)
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

// GetBuildStatus 获取构建状态
// GET /linuxlab/build/status
func (h *Handler) GetBuildStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"building": h.service.IsBuilding(),
		"running":  h.service.IsRunning(),
	})
}

// Boot 启动虚拟开发板（SSE 流式输出）
// POST /linuxlab/boot
func (h *Handler) Boot(c *gin.Context) {
	var req BootRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if h.service.IsRunning() {
		c.JSON(http.StatusConflict, gin.H{"error": "已有虚拟板正在运行"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	progressChan := make(chan ProgressEvent, 100)

	go func() {
		h.service.Boot(req.Board, progressChan)
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

// StopBoot 停止虚拟开发板
// DELETE /linuxlab/boot
func (h *Handler) StopBoot(c *gin.Context) {
	if err := h.service.StopBoot(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "虚拟板已停止"})
}

// ExecMakeTarget 执行任意 make 目标（高级模式）
// POST /linuxlab/make
func (h *Handler) ExecMakeTarget(c *gin.Context) {
	var req MakeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target 不能为空"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	progressChan := make(chan ProgressEvent, 100)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		h.service.ExecMake(ctx, req.Target, req.Board, progressChan)
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
