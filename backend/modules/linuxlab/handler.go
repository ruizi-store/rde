package linuxlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

// Build 触发构建（SSE 流式输出；断开后任务继续，可用 /build/logs 重连）
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

	job, err := h.service.StartBuild(target, req.Board)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	h.streamBuildJob(c, job, 0)
}

// StreamBuildLogs 重连构建日志（历史 + 实时）
// GET /linuxlab/build/logs?since=0
func (h *Handler) StreamBuildLogs(c *gin.Context) {
	job := h.service.CurrentJob()
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有可重连的构建任务"})
		return
	}

	since := int64(0)
	if q := c.Query("since"); q != "" {
		if v, err := strconv.ParseInt(q, 10, 64); err == nil && v >= 0 {
			since = v
		}
	}

	h.streamBuildJob(c, job, since)
}

// streamBuildJob 将 job 日志以 SSE 推送；客户端断开不影响构建
func (h *Handler) streamBuildJob(c *gin.Context, job *BuildJob, since int64) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ch, cancel := job.Subscribe(since)
	defer cancel()

	clientGone := c.Request.Context().Done()
	lastSeq := since

	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			return false
		case ev, ok := <-ch:
			if !ok {
				fmt.Fprintf(w, "event: done\ndata: {\"status\":\"done\"}\n\n")
				return false
			}
			if ev.Done {
				data, _ := json.Marshal(gin.H{"status": "done", "build_status": ev.Status})
				fmt.Fprintf(w, "event: done\ndata: %s\n\n", data)
				return false
			}
			if ev.Seq > 0 && ev.Seq <= lastSeq {
				return true // 去重
			}
			if ev.Seq > lastSeq {
				lastSeq = ev.Seq
			}
			payload := ProgressEvent{Status: ev.Status, Message: ev.Message, Line: ev.Line}
			data, _ := json.Marshal(struct {
				ProgressEvent
				Seq int64 `json:"seq,omitempty"`
			}{ProgressEvent: payload, Seq: ev.Seq})
			fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
			return true
		}
	})
}

// GetBuildStatus 获取构建状态
// GET /linuxlab/build/status
func (h *Handler) GetBuildStatus(c *gin.Context) {
	info := h.service.GetBuildInfo()
	c.JSON(http.StatusOK, gin.H{
		"building":        info.Building,
		"running":         h.service.IsRunning(),
		"board":           info.Board,
		"target":          info.Target,
		"last_seq":        info.LastSeq,
		"status":          info.Status,
		"job_id":          info.JobID,
		"logs_available":  info.JobID != "",
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
