// Package flatpak HTTP 处理器
package flatpak

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器实例
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	fp := r.Group("/flatpak")
	{
		// 环境检测与安装
		fp.GET("/setup/status", h.GetSetupStatus)
		fp.POST("/setup/run", h.RunSetupStream)

		// 桌面管理
		fp.GET("/desktop/status", h.GetDesktopStatus)
		fp.POST("/desktop/start", h.StartDesktop)
		fp.POST("/desktop/stop", h.StopDesktop)
		fp.POST("/desktop/restart", h.RestartDesktop)
		fp.GET("/desktop/config", h.GetDesktopConfig)
		fp.PUT("/desktop/config", h.UpdateDesktopConfig)
		fp.POST("/desktop/clipboard", h.SetClipboard)
		fp.GET("/desktop/clipboard", h.GetClipboard)

		// 应用管理
		fp.GET("/apps", h.GetInstalledApps)
		fp.GET("/apps/search", h.SearchApps)
		fp.GET("/apps/recommended", h.GetRecommendedApps)
		fp.GET("/apps/categories", h.GetRecommendedCategories)
		fp.POST("/apps/install-stream", h.InstallAppStream)
		fp.GET("/apps/installing", h.GetActiveInstalls)
		fp.GET("/apps/install-progress/:appid", h.WatchInstallProgress)
		fp.POST("/apps/uninstall", h.UninstallApp)
		fp.POST("/apps/run", h.RunApp)

		// 图标
		fp.GET("/icons/:appid", h.ServeIcon)

		// KasmVNC 反向代理
		fp.Any("/vnc/*path", h.ProxyVNC)
	}
}

// ==================== Setup Handlers ====================

// getUsername 从认证上下文提取用户名
func (h *Handler) getUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return "", false
	}
	return username.(string), true
}

// GetSetupStatus 获取环境检测状态
func (h *Handler) GetSetupStatus(c *gin.Context) {
	status := h.service.GetSetupStatus()
	c.JSON(http.StatusOK, status)
}

// RunSetupStream SSE 流式执行环境安装
func (h *Handler) RunSetupStream(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	sendEvent(c.Writer, flusher, "start", "")

	done := make(chan error, 1)

	h.service.RunSetup(username,
		func(line string) {
			sendEvent(c.Writer, flusher, "progress", line)
		},
		func(err error) {
			done <- err
		},
	)

	err := <-done
	if err != nil {
		sendEvent(c.Writer, flusher, "error", err.Error())
	} else {
		sendEvent(c.Writer, flusher, "complete", "")
	}
}

// ==================== Desktop Handlers ====================

// GetDesktopStatus 获取桌面状态
func (h *Handler) GetDesktopStatus(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	status := h.service.GetDesktopStatus(username)
	c.JSON(http.StatusOK, status)
}

// StartDesktop 启动桌面
func (h *Handler) StartDesktop(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	if err := h.service.StartDesktop(username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "started"})
}

// StopDesktop 停止桌面
func (h *Handler) StopDesktop(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	h.service.StopDesktop(username)
	c.JSON(http.StatusOK, gin.H{"message": "stopped"})
}

// RestartDesktop 重启桌面
func (h *Handler) RestartDesktop(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	if err := h.service.RestartDesktop(username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "restarted"})
}

// GetDesktopConfig 获取桌面配置
func (h *Handler) GetDesktopConfig(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	config := h.service.GetDesktopConfig(username)
	c.JSON(http.StatusOK, config)
}

// UpdateDesktopConfig 更新桌面配置
func (h *Handler) UpdateDesktopConfig(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	var config DesktopConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.service.UpdateDesktopConfig(username, config)
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// SetClipboard 设置桌面剪贴板
func (h *Handler) SetClipboard(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.SetClipboard(username, req.Text); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GetClipboard 获取桌面剪贴板
func (h *Handler) GetClipboard(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}
	text, err := h.service.GetClipboard(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": text})
}

// ==================== App Handlers ====================

// GetInstalledApps 获取已安装的 Flatpak 应用
func (h *Handler) GetInstalledApps(c *gin.Context) {
	apps := h.service.GetInstalledApps()
	c.JSON(http.StatusOK, apps)
}

// SearchApps 搜索 Flathub 应用
func (h *Handler) SearchApps(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}
	apps := h.service.SearchApps(query, 50)
	c.JSON(http.StatusOK, apps)
}

// GetRecommendedApps 获取推荐应用列表
func (h *Handler) GetRecommendedApps(c *gin.Context) {
	category := c.Query("category")
	apps := h.service.GetRecommendedApps(category)
	c.JSON(http.StatusOK, apps)
}

// GetRecommendedCategories 获取推荐分类
func (h *Handler) GetRecommendedCategories(c *gin.Context) {
	cats := h.service.GetRecommendedCategories()
	c.JSON(http.StatusOK, cats)
}

// InstallAppStream SSE 流式安装应用
func (h *Handler) InstallAppStream(c *gin.Context) {
	var req InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	appName := req.AppName
	if appName == "" {
		appName = req.AppID
	}

	sendEvent(c.Writer, flusher, "start", req.AppID)

	done := make(chan error, 1)

	h.service.InstallApp(req.AppID, appName,
		func(line string) {
			sendEvent(c.Writer, flusher, "progress", line)
		},
		func(err error) {
			done <- err
		},
	)

	err := <-done
	if err != nil {
		sendEvent(c.Writer, flusher, "error", err.Error())
	} else {
		sendEvent(c.Writer, flusher, "complete", req.AppID)
	}
}

// GetActiveInstalls 获取当前正在安装的应用列表
func (h *Handler) GetActiveInstalls(c *gin.Context) {
	installs := h.service.GetActiveInstalls()
	if installs == nil {
		installs = []ActiveInstall{}
	}
	c.JSON(http.StatusOK, installs)
}

// WatchInstallProgress SSE 重连安装进度
func (h *Handler) WatchInstallProgress(c *gin.Context) {
	appID := c.Param("appid")

	logs, ch, status, errMsg, exists := h.service.WatchInstallProgress(appID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active install for " + appID})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// 发送历史日志
	for _, line := range logs {
		sendEvent(c.Writer, flusher, "progress", line)
	}

	// 如果已经完成，直接发送结果
	if status != "installing" {
		if status == "error" {
			sendEvent(c.Writer, flusher, "error", errMsg)
		} else {
			sendEvent(c.Writer, flusher, "complete", appID)
		}
		return
	}

	// 实时接收新日志
	defer h.service.UnsubscribeInstall(appID, ch)
	for line := range ch {
		sendEvent(c.Writer, flusher, "progress", line)
	}

	// 通道关闭后查询最终状态
	_, _, finalStatus, finalErr, _ := h.service.WatchInstallProgress(appID)
	if finalStatus == "error" {
		sendEvent(c.Writer, flusher, "error", finalErr)
	} else {
		sendEvent(c.Writer, flusher, "complete", appID)
	}
}

// UninstallApp 卸载应用
func (h *Handler) UninstallApp(c *gin.Context) {
	var req InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UninstallApp(req.AppID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "uninstalled"})
}

// RunApp 运行应用
func (h *Handler) RunApp(c *gin.Context) {
	var req RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从认证上下文获取当前登录用户名
	username, ok := h.getUsername(c)
	if !ok {
		return
	}

	if err := h.service.RunApp(req.AppID, req.Args, username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "launched"})
}

// ==================== Icon Handler ====================

// ServeIcon 提供 Flatpak 应用图标
func (h *Handler) ServeIcon(c *gin.Context) {
	appID := c.Param("appid")
	if appID == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// 查找图标文件
	iconPaths := []string{
		fmt.Sprintf("/var/lib/flatpak/exports/share/icons/hicolor/128x128/apps/%s.png", appID),
		fmt.Sprintf("/var/lib/flatpak/exports/share/icons/hicolor/64x64/apps/%s.png", appID),
		fmt.Sprintf("/var/lib/flatpak/exports/share/icons/hicolor/scalable/apps/%s.svg", appID),
		fmt.Sprintf("/var/lib/flatpak/exports/share/icons/hicolor/256x256/apps/%s.png", appID),
		fmt.Sprintf("/var/lib/flatpak/exports/share/icons/hicolor/512x512/apps/%s.png", appID),
		fmt.Sprintf("%s/.local/share/flatpak/exports/share/icons/hicolor/128x128/apps/%s.png", os.Getenv("HOME"), appID),
		fmt.Sprintf("%s/.local/share/flatpak/exports/share/icons/hicolor/64x64/apps/%s.png", os.Getenv("HOME"), appID),
	}

	for _, p := range iconPaths {
		if _, err := os.Stat(p); err == nil {
			c.File(p)
			return
		}
	}

	c.Status(http.StatusNotFound)
}

// ==================== KasmVNC 反向代理 ====================

var vncWSUpgrader = websocket.Upgrader{
	CheckOrigin:  func(r *http.Request) bool { return true },
	Subprotocols: []string{"binary"},
}

// ProxyVNC 反向代理 KasmVNC HTTP 和 WebSocket 请求
func (h *Handler) ProxyVNC(c *gin.Context) {
	username, ok := h.getUsername(c)
	if !ok {
		return
	}

	path := c.Param("path")
	if path == "" {
		path = "/"
	}

	// 获取用户的桌面状态
	status := h.service.GetDesktopStatus(username)
	if !status.Running {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "desktop not running"})
		return
	}

	targetURL := fmt.Sprintf("http://127.0.0.1:%d", status.WebSocketPort)

	// WebSocket 升级
	if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		h.proxyVNCWebSocket(c, status.WebSocketPort, path)
		return
	}

	// 将 token 写入 cookie，这样 KasmVNC JS 客户端打开 WebSocket 时会自动携带
	if token := c.Query("token"); token != "" {
		c.SetCookie("token", token, 86400, "/api/v1/flatpak/vnc/", "", false, false)
	}

	// 普通 HTTP 反向代理
	target, _ := url.Parse(targetURL)
	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = path
		req.URL.RawQuery = c.Request.URL.RawQuery
		req.Host = target.Host
		// 删除 Referer/Origin，KasmVNC 会根据这些头返回 401
		req.Header.Del("Referer")
		req.Header.Del("Origin")
	}

	// KasmVNC web client 不缓存，允许 iframe 嵌入
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		resp.Header.Set("Pragma", "no-cache")
		resp.Header.Set("Expires", "0")
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("X-Frame-Options")
		// 防止 KasmVNC 返回的 WWW-Authenticate 头触发浏览器原生 Basic Auth 弹窗
		resp.Header.Del("WWW-Authenticate")
		return nil
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

// proxyVNCWebSocket 代理 KasmVNC WebSocket 连接
func (h *Handler) proxyVNCWebSocket(c *gin.Context, port int, path string) {
	// 升级前端连接
	frontendConn, err := vncWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer frontendConn.Close()

	// 连接到 KasmVNC WebSocket
	backendURL := fmt.Sprintf("ws://127.0.0.1:%d%s", port, path)
	if c.Request.URL.RawQuery != "" {
		backendURL += "?" + c.Request.URL.RawQuery
	}

	dialer := websocket.Dialer{
		Subprotocols: []string{"binary"},
	}
	// KasmVNC 要求 WebSocket 请求必须携带 Origin 头，否则返回 404
	wsHeaders := http.Header{}
	wsHeaders.Set("Origin", fmt.Sprintf("http://127.0.0.1:%d", port))
	backendConn, _, err := dialer.Dial(backendURL, wsHeaders)
	if err != nil {
		frontendConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "backend connection failed"))
		return
	}
	defer backendConn.Close()

	// 双向转发
	done := make(chan struct{})

	// 前端 -> 后端
	go func() {
		defer close(done)
		for {
			msgType, msg, err := frontendConn.ReadMessage()
			if err != nil {
				return
			}
			if err := backendConn.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()

	// 后端 -> 前端
	go func() {
		for {
			msgType, msg, err := backendConn.ReadMessage()
			if err != nil {
				frontendConn.Close()
				return
			}
			if err := frontendConn.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()

	<-done
}

// ==================== SSE 辅助 ====================

// sendEvent 发送 SSE 事件
func sendEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, message string) {
	escaped := strings.ReplaceAll(message, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	fmt.Fprintf(w, "data: {\"type\":\"%s\",\"message\":\"%s\"}\n\n", eventType, escaped)
	flusher.Flush()
}
