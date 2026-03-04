// Package android HTTP 处理器
package android

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Handler HTTP 处理器
type Handler struct {
	service       *Service
	installWizard *InstallWizard
	logger        *zap.Logger
	upgrader      websocket.Upgrader
}

// NewHandler 创建处理器实例
func NewHandler(service *Service, installWizard *InstallWizard, logger *zap.Logger) *Handler {
	return &Handler{
		service:       service,
		installWizard: installWizard,
		logger:        logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	android := r.Group("/android")
	{
		// 设备管理
		android.GET("/devices", h.GetDevices)
		android.POST("/devices/connect", h.ConnectDevice)
		android.POST("/devices/disconnect", h.DisconnectDevice)

		// 单设备操作
		device := android.Group("/devices/:serial")
		{
			device.GET("", h.GetDevice)
			device.GET("/apps", h.GetApps)
			device.POST("/apps/install", h.InstallAppBySerial)
			device.POST("/apps/uninstall", h.UninstallApp)
			device.POST("/apps/:package/launch", h.LaunchApp)
			device.POST("/apps/:package/stop", h.StopApp)
			device.POST("/reboot", h.RebootDevice)
			device.POST("/shell", h.Shell)
			device.POST("/screenshot", h.Screenshot)
			device.POST("/input", h.InputDevice)
			device.GET("/files", h.ListFiles)
			device.POST("/files/push", h.PushFile)
			device.POST("/files/pull", h.PullFile)
		}

		// 投屏会话
		android.GET("/session", h.GetSession)
		android.POST("/session/start", h.StartSession)
		android.POST("/session/stop", h.StopSession)

		// 控制
		android.POST("/control", h.SendControl)
		android.GET("/config", h.GetConfig)

		// WebSocket 投屏流
		android.GET("/ws/screen", h.HandleWebSocket)

		// APK 管理
		android.POST("/apk/parse", h.ParseAPK)
		android.POST("/apk/install", h.InstallAPK)
		android.POST("/apk/upload", h.UploadAPK)

		// 环境安装
		android.GET("/env/status", h.GetInstallStatus)
		android.POST("/env/install", h.StartInstall)
		android.POST("/env/cancel", h.CancelInstall)
		android.GET("/env/steps", h.GetInstallSteps)
		android.GET("/ws/install", h.HandleInstallWebSocket)

		// 容器管理
		android.POST("/container/start", h.StartContainer)
		android.POST("/container/stop", h.StopContainer)
	}
}

// ==================== 设备管理 API ====================

// GetDevices 获取设备列表
func (h *Handler) GetDevices(c *gin.Context) {
	devices, err := h.service.GetDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    devices,
	})
}

// ConnectDevice 连接设备
func (h *Handler) ConnectDevice(c *gin.Context) {
	var req struct {
		Serial string `json:"serial" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.service.ConnectDevice(req.Serial); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
	})
}

// DisconnectDevice 断开设备
func (h *Handler) DisconnectDevice(c *gin.Context) {
	var req struct {
		Serial string `json:"serial" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.service.DisconnectDevice(req.Serial); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
	})
}

// ==================== 单设备操作 API ====================

// GetDevice 获取单个设备信息
func (h *Handler) GetDevice(c *gin.Context) {
	serial := c.Param("serial")
	device, err := h.service.GetDevice(serial)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: device})
}

// GetApps 获取设备上已安装的应用列表
func (h *Handler) GetApps(c *gin.Context) {
	serial := c.Param("serial")
	apps, err := h.service.GetApps(serial)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: apps})
}

// InstallAppBySerial 通过设备序列号安装 APK
func (h *Handler) InstallAppBySerial(c *gin.Context) {
	serial := c.Param("serial")
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request: " + err.Error()})
		return
	}
	realPath := h.resolveVirtualPath(c, req.Path)
	app, err := h.service.InstallAPKToDevice(serial, realPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: app})
}

// UninstallApp 卸载应用
func (h *Handler) UninstallApp(c *gin.Context) {
	serial := c.Param("serial")
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request: " + err.Error()})
		return
	}
	if err := h.service.UninstallApp(serial, req.Package); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

// LaunchApp 启动应用
func (h *Handler) LaunchApp(c *gin.Context) {
	serial := c.Param("serial")
	pkg := c.Param("package")
	if err := h.service.LaunchApp(serial, pkg); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

// StopApp 停止应用
func (h *Handler) StopApp(c *gin.Context) {
	serial := c.Param("serial")
	pkg := c.Param("package")
	if err := h.service.StopApp(serial, pkg); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

// RebootDevice 重启设备
func (h *Handler) RebootDevice(c *gin.Context) {
	serial := c.Param("serial")
	if err := h.service.RebootDevice(serial); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

// Shell 执行 Shell 命令
func (h *Handler) Shell(c *gin.Context) {
	serial := c.Param("serial")
	var req struct {
		Command string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request: " + err.Error()})
		return
	}
	output, err := h.service.Shell(serial, req.Command)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]string{"output": output}})
}

// Screenshot 截图
func (h *Handler) Screenshot(c *gin.Context) {
	serial := c.Param("serial")
	image, err := h.service.Screenshot(serial)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]string{"image": image}})
}

// InputDevice 向设备发送输入
func (h *Handler) InputDevice(c *gin.Context) {
	serial := c.Param("serial")
	var req struct {
		Type string `json:"type" binding:"required"`
		Args string `json:"args" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request: " + err.Error()})
		return
	}
	if err := h.service.Input(serial, req.Type, req.Args); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

// ListFiles 列出设备文件
func (h *Handler) ListFiles(c *gin.Context) {
	serial := c.Param("serial")
	path := c.DefaultQuery("path", "/sdcard")
	files, err := h.service.ListFiles(serial, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: files})
}

// PushFile 推送文件到设备
func (h *Handler) PushFile(c *gin.Context) {
	serial := c.Param("serial")
	var req struct {
		LocalPath  string `json:"local_path" binding:"required"`
		RemotePath string `json:"remote_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request: " + err.Error()})
		return
	}
	realPath := h.resolveVirtualPath(c, req.LocalPath)
	if err := h.service.PushFile(serial, realPath, req.RemotePath); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

// PullFile 从设备拉取文件
func (h *Handler) PullFile(c *gin.Context) {
	serial := c.Param("serial")
	var req struct {
		RemotePath string `json:"remote_path" binding:"required"`
		LocalPath  string `json:"local_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid request: " + err.Error()})
		return
	}
	if err := h.service.PullFile(serial, req.RemotePath, req.LocalPath); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

// ==================== 投屏会话 API ====================

// GetSession 获取当前会话
func (h *Handler) GetSession(c *gin.Context) {
	session := h.service.GetCurrentSession()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    session,
	})
}

// StartSession 启动会话
func (h *Handler) StartSession(c *gin.Context) {
	var req struct {
		Serial string        `json:"serial" binding:"required"`
		Config *ScrcpyConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	session, err := h.service.StartSession(req.Serial, req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    session,
	})
}

// StopSession 停止会话
func (h *Handler) StopSession(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId"`
	}

	c.ShouldBindJSON(&req)

	if err := h.service.StopSession(req.SessionID); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
	})
}

// GetConfig 获取默认配置
func (h *Handler) GetConfig(c *gin.Context) {
	config := DefaultScrcpyConfig()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    config,
	})
}

// SendControl 发送控制消息
func (h *Handler) SendControl(c *gin.Context) {
	var msg ControlMessage

	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid control message: " + err.Error(),
		})
		return
	}

	if err := h.service.SendControl(msg); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
	})
}

// ==================== WebSocket ====================

// HandleWebSocket 处理投屏 WebSocket 连接
func (h *Handler) HandleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	client := &ClientConn{
		Conn: conn,
	}

	h.service.AddClient(client)
	defer func() {
		h.service.RemoveClient(client)
		client.Close()
	}()

	// 发送当前会话信息
	session := h.service.GetCurrentSession()
	if session != nil {
		client.Send(WSMessage{
			Type:    MsgTypeDeviceInfo,
			Payload: session,
		})
	}

	// 读取控制消息
	for {
		var msg ControlMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Debug("WebSocket closed", zap.Error(err))
			}
			break
		}

		if err := h.service.SendControl(msg); err != nil {
			client.Send(WSMessage{
				Type:  MsgTypeError,
				Error: err.Error(),
			})
		}
	}
}

// ==================== APK 管理 API ====================

// resolveVirtualPath 将虚拟路径解析为真实文件系统路径
func (h *Handler) resolveVirtualPath(c *gin.Context, virtualPath string) string {
	if _, err := os.Stat(virtualPath); err == nil {
		return virtualPath
	}

	username := c.GetHeader("X-RuiziOS-User")
	dataRoot := "/var/lib/ruizios/data"
	virtualPath = filepath.Clean(virtualPath)

	if strings.HasPrefix(virtualPath, "/home/") {
		subPath := strings.TrimPrefix(virtualPath, "/home/")
		if username != "" {
			return filepath.Join(dataRoot, "home", username, subPath)
		}
		return filepath.Join(dataRoot, "home", subPath)
	}

	if strings.HasPrefix(virtualPath, "/") {
		return filepath.Join(dataRoot, virtualPath)
	}

	return virtualPath
}

// ParseAPK 解析 APK 文件
func (h *Handler) ParseAPK(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	realPath := h.resolveVirtualPath(c, req.Path)
	info, err := h.service.ParseAPK(realPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    info,
	})
}

// InstallAPK 安装 APK 文件
func (h *Handler) InstallAPK(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	realPath := h.resolveVirtualPath(c, req.Path)
	app, err := h.service.InstallAPK(realPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    app,
	})
}

// UploadAPK 上传并安装 APK 文件
func (h *Handler) UploadAPK(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "未找到上传文件: " + err.Error(),
		})
		return
	}

	// 验证文件扩展名
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".apk") {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "只支持 APK 文件",
		})
		return
	}

	// 创建临时目录
	tmpDir := filepath.Join(os.TempDir(), "rde-apk-upload")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "创建临时目录失败: " + err.Error(),
		})
		return
	}

	// 保存到临时文件
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename))
	if err := c.SaveUploadedFile(file, tmpFile); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "保存文件失败: " + err.Error(),
		})
		return
	}

	// 安装 APK
	app, err := h.service.InstallAPK(tmpFile)
	
	// 清理临时文件
	os.Remove(tmpFile)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    app,
	})
}

// ==================== 安装向导 API ====================

// GetInstallStatus 获取安装状态
func (h *Handler) GetInstallStatus(c *gin.Context) {
	status := h.installWizard.CheckEnvironment()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

// StartInstall 启动安装
func (h *Handler) StartInstall(c *gin.Context) {
	if h.installWizard.IsRunning() {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "安装已在进行中",
		})
		return
	}

	if err := h.installWizard.Start(context.Background()); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "安装已启动",
			"steps":   h.installWizard.GetSteps(),
		},
	})
}

// CancelInstall 取消安装
func (h *Handler) CancelInstall(c *gin.Context) {
	h.installWizard.Cancel()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
	})
}

// GetInstallSteps 获取安装步骤
func (h *Handler) GetInstallSteps(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"steps":      h.installWizard.GetSteps(),
			"is_running": h.installWizard.IsRunning(),
			"current":    h.installWizard.GetCurrentStep(),
		},
	})
}

// HandleInstallWebSocket 处理安装进度 WebSocket
func (h *Handler) HandleInstallWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Install WebSocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	updates := make(chan *StepInfo, 10)
	done := make(chan struct{})

	h.installWizard.OnStepUpdate(func(step *StepInfo) {
		select {
		case updates <- step:
		default:
		}
	})

	// 发送当前状态
	conn.WriteJSON(map[string]interface{}{
		"type": "init",
		"data": map[string]interface{}{
			"status":     h.installWizard.CheckEnvironment(),
			"steps":      h.installWizard.GetSteps(),
			"is_running": h.installWizard.IsRunning(),
		},
	})

	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				close(done)
				return
			}
		}
	}()

	for {
		select {
		case step := <-updates:
			if err := conn.WriteJSON(map[string]interface{}{
				"type": "step_update",
				"data": step,
			}); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

// ==================== 容器管理 API ====================

// StartContainer 启动容器
func (h *Handler) StartContainer(c *gin.Context) {
	if err := h.service.StartContainer(); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
	})
}

// StopContainer 停止容器
func (h *Handler) StopContainer(c *gin.Context) {
	if err := h.service.StopContainer(); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
	})
}
