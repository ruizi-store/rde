// Package system 提供系统信息 HTTP 处理器
package system

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ruizi-store/rde/backend/model"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type response struct {
	Success int         `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, response{
		Success: 200,
		Message: "success",
		Data:    data,
	})
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, response{
		Success: code,
		Message: msg,
	})
}

// GetSystemInfo 获取系统信息
// @Summary 获取系统信息
// @Tags system
// @Produce json
// @Success 200 {object} SystemInfo
// @Router /api/v1/system/info [get]
func (h *Handler) GetSystemInfo(c *gin.Context) {
	info, err := h.service.GetSystemInfo(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, info)
}

// GetCPUInfo 获取 CPU 信息
// @Summary 获取 CPU 信息
// @Tags system
// @Produce json
// @Success 200 {object} CPUInfo
// @Router /api/v1/system/cpu [get]
func (h *Handler) GetCPUInfo(c *gin.Context) {
	info, err := h.service.GetCPUInfo(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, info)
}

// GetMemoryInfo 获取内存信息
// @Summary 获取内存信息
// @Tags system
// @Produce json
// @Success 200 {object} MemoryInfo
// @Router /api/v1/system/memory [get]
func (h *Handler) GetMemoryInfo(c *gin.Context) {
	info, err := h.service.GetMemoryInfo(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, info)
}

// GetDiskInfo 获取磁盘信息
// @Summary 获取磁盘信息
// @Tags system
// @Produce json
// @Param path query string false "磁盘路径"
// @Success 200 {object} DiskInfo
// @Router /api/v1/system/disk [get]
func (h *Handler) GetDiskInfo(c *gin.Context) {
	path := c.Query("path")
	info, err := h.service.GetDiskInfo(c.Request.Context(), path)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, info)
}

// GetAllDisks 获取所有磁盘信息
// @Summary 获取所有磁盘信息
// @Tags system
// @Produce json
// @Success 200 {array} DiskInfo
// @Router /api/v1/system/disks [get]
func (h *Handler) GetAllDisks(c *gin.Context) {
	disks, err := h.service.GetAllDisks(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, disks)
}

// GetNetworkInterfaces 获取网络接口
// @Summary 获取网络接口
// @Tags system
// @Produce json
// @Param physical query bool false "仅物理网卡"
// @Success 200 {array} NetworkInterface
// @Router /api/v1/system/network/interfaces [get]
func (h *Handler) GetNetworkInterfaces(c *gin.Context) {
	physical := c.Query("physical") == "true"
	interfaces, err := h.service.GetNetworkInterfaces(c.Request.Context(), physical)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, interfaces)
}

// GetNetworkStats 获取网络统计
// @Summary 获取网络统计
// @Tags system
// @Produce json
// @Success 200 {array} NetworkStats
// @Router /api/v1/system/network/stats [get]
func (h *Handler) GetNetworkStats(c *gin.Context) {
	stats, err := h.service.GetNetworkStats(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, stats)
}

// GetDeviceInfo 获取设备信息
// @Summary 获取设备信息
// @Tags system
// @Produce json
// @Success 200 {object} DeviceInfo
// @Router /api/v1/system/device [get]
func (h *Handler) GetDeviceInfo(c *gin.Context) {
	info, err := h.service.GetDeviceInfo(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, info)
}

// GetResourceUsage 获取资源使用情况
// @Summary 获取当前资源使用情况
// @Tags system
// @Produce json
// @Success 200 {object} ResourceUsage
// @Router /api/v1/system/usage [get]
func (h *Handler) GetResourceUsage(c *gin.Context) {
	usage, err := h.service.GetResourceUsage(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, usage)
}

// GetTimeZone 获取时区
// @Summary 获取时区信息
// @Tags system
// @Produce json
// @Success 200 {object} TimeZoneInfo
// @Router /api/v1/system/timezone [get]
func (h *Handler) GetTimeZone(c *gin.Context) {
	tz := h.service.GetTimeZone(c.Request.Context())
	ok(c, tz)
}

// GetTopProcesses 获取进程列表
// @Summary 获取 CPU/内存占用最高的进程
// @Tags system
// @Produce json
// @Param limit query int false "返回数量" default(10)
// @Param sort query string false "排序字段" Enums(cpu, memory)
// @Success 200 {array} ProcessInfo
// @Router /api/v1/system/processes [get]
func (h *Handler) GetTopProcesses(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sort", "cpu")

	procs, err := h.service.GetTopProcesses(c.Request.Context(), limit, sortBy)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, procs)
}

// GetHealthStatus 获取健康状态
// @Summary 获取系统健康状态
// @Tags system
// @Produce json
// @Success 200 {object} HealthStatus
// @Router /api/v1/system/health [get]
func (h *Handler) GetHealthStatus(c *gin.Context) {
	status := h.service.GetHealthStatus(c.Request.Context())
	ok(c, status)
}

// GetLogs 获取日志
// @Summary 获取系统日志
// @Tags system
// @Produce json
// @Param lines query int false "行数" default(100)
// @Success 200 {array} string
// @Router /api/v1/system/logs [get]
func (h *Handler) GetLogs(c *gin.Context) {
	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "100"))

	logs, err := h.service.GetLogs(c.Request.Context(), "", lines)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, logs)
}

// Reboot 重启系统
// @Summary 重启系统
// @Tags system
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/system/reboot [post]
func (h *Handler) Reboot(c *gin.Context) {
	if err := h.service.Reboot(c.Request.Context()); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// Shutdown 关机
// @Summary 关闭系统
// @Tags system
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/system/shutdown [post]
func (h *Handler) Shutdown(c *gin.Context) {
	if err := h.service.Shutdown(c.Request.Context()); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, nil)
}

// GetNetwork 获取网络信息（包含接口和统计）
// @Summary 获取网络信息
// @Tags system
// @Produce json
// @Success 200 {object} NetworkInfo
// @Router /api/v1/system/network [get]
func (h *Handler) GetNetwork(c *gin.Context) {
	interfaces, err := h.service.GetNetworkInterfaces(c.Request.Context(), false)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	stats, err := h.service.GetNetworkStats(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, map[string]interface{}{
		"interfaces": interfaces,
		"stats":      stats,
	})
}

// GetSSHStatus 获取 SSH 服务状态
// @Summary 获取 SSH 服务状态
// @Tags system
// @Produce json
// @Success 200 {object} SSHStatus
// @Router /api/v1/system/ssh [get]
func (h *Handler) GetSSHStatus(c *gin.Context) {
	status, err := h.service.GetSSHStatus(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, status)
}

// EnableSSH 启用 SSH 服务
// @Summary 启用 SSH 服务
// @Tags system
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/system/ssh/enable [post]
func (h *Handler) EnableSSH(c *gin.Context) {
	if err := h.service.EnableSSH(c.Request.Context()); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, map[string]interface{}{
		"message": "SSH service enabled",
	})
}

// DisableSSH 禁用 SSH 服务
// @Summary 禁用 SSH 服务
// @Tags system
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/system/ssh/disable [post]
func (h *Handler) DisableSSH(c *gin.Context) {
	if err := h.service.DisableSSH(c.Request.Context()); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, map[string]interface{}{
		"message": "SSH service disabled",
	})
}

// SetSSHAutoStart 设置 SSH 开机自启
// @Summary 设置 SSH 开机自启
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} response
// @Router /api/v1/system/ssh/autostart [post]
func (h *Handler) SetSSHAutoStart(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if err := h.service.SetSSHAutoStart(c.Request.Context(), req.Enabled); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, map[string]interface{}{
		"message": "SSH autostart updated",
		"enabled": req.Enabled,
	})
}

// GetRemoteAccessSettings 获取远程访问设置
// @Summary 获取远程访问设置
// @Tags system
// @Produce json
// @Success 200 {object} RemoteAccessSettings
// @Router /api/v1/system/security/remote-access [get]
func (h *Handler) GetRemoteAccessSettings(c *gin.Context) {
	settings, err := h.service.GetRemoteAccessSettings(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, settings)
}

// UpdateRemoteAccessSettings 更新远程访问设置
// @Summary 更新远程访问设置
// @Tags system
// @Accept json
// @Produce json
// @Param request body UpdateRemoteAccessRequest true "更新请求"
// @Success 200 {object} response
// @Router /api/v1/system/security/remote-access [put]
func (h *Handler) UpdateRemoteAccessSettings(c *gin.Context) {
	var req struct {
		SSHEnabled      *bool  `json:"ssh_enabled"`
		TerminalEnabled *bool  `json:"terminal_enabled"`
		Password        string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "请输入密码确认操作")
		return
	}

	// TODO: 验证密码（需要从 users 模块获取用户信息并验证）
	// 这里暂时跳过密码验证，实际需要调用 users 模块
	if req.Password == "" {
		fail(c, 400, "请输入密码确认操作")
		return
	}

	ctx := c.Request.Context()

	// 更新 SSH 设置
	if req.SSHEnabled != nil {
		if err := h.service.SetSSHEnabled(ctx, *req.SSHEnabled); err != nil {
			fail(c, 500, "设置 SSH 失败: "+err.Error())
			return
		}
	}

	// 更新终端设置
	if req.TerminalEnabled != nil {
		if err := h.service.SetTerminalEnabled(ctx, *req.TerminalEnabled); err != nil {
			fail(c, 500, "设置终端失败: "+err.Error())
			return
		}
	}

	// 返回更新后的设置
	settings, err := h.service.GetRemoteAccessSettings(ctx)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}

	ok(c, map[string]interface{}{
		"message":  "设置已更新",
		"settings": settings,
	})
}

// ==================== 代理设置 ====================

// GetProxyConfig 获取代理配置
// @Summary 获取代理配置
// @Tags system
// @Produce json
// @Success 200 {object} model.SystemProxyConfig
// @Router /api/v1/system/proxy [get]
func (h *Handler) GetProxyConfig(c *gin.Context) {
	config, err := h.service.GetProxyConfig(c.Request.Context())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, config)
}

// SaveProxyConfig 保存代理配置
// @Summary 保存代理配置
// @Tags system
// @Accept json
// @Produce json
// @Param config body model.SystemProxyConfig true "代理配置"
// @Success 200 {object} response
// @Router /api/v1/system/proxy [post]
func (h *Handler) SaveProxyConfig(c *gin.Context) {
	var config model.SystemProxyConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	if err := h.service.SaveProxyConfig(c.Request.Context(), &config); err != nil {
		fail(c, 500, err.Error())
		return
	}

	ok(c, map[string]interface{}{
		"message": "代理设置已保存",
	})
}

// TestProxy 测试代理连接
// @Summary 测试代理连接
// @Tags system
// @Accept json
// @Produce json
// @Param request body model.ProxyTestRequest true "测试请求"
// @Success 200 {object} model.ProxyTestResponse
// @Router /api/v1/system/proxy/test [post]
func (h *Handler) TestProxy(c *gin.Context) {
	var req model.ProxyTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误: "+err.Error())
		return
	}

	result, err := h.service.TestProxy(c.Request.Context(), &req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}

	ok(c, result)
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	sys := group.Group("/system")
	{
		sys.GET("/info", h.GetSystemInfo)
		sys.GET("/cpu", h.GetCPUInfo)
		sys.GET("/memory", h.GetMemoryInfo)
		sys.GET("/disk", h.GetDiskInfo)
		sys.GET("/disks", h.GetAllDisks)
		sys.GET("/network", h.GetNetwork)
		sys.GET("/network/interfaces", h.GetNetworkInterfaces)
		sys.GET("/network/stats", h.GetNetworkStats)
		sys.GET("/device", h.GetDeviceInfo)
		sys.GET("/usage", h.GetResourceUsage)
		sys.GET("/timezone", h.GetTimeZone)
		sys.GET("/processes", h.GetTopProcesses)
		sys.GET("/health", h.GetHealthStatus)
		sys.GET("/logs", h.GetLogs)
		sys.POST("/reboot", h.Reboot)
		sys.POST("/shutdown", h.Shutdown)
		// SSH 服务管理（保留旧接口兼容）
		sys.GET("/ssh", h.GetSSHStatus)
		sys.POST("/ssh/enable", h.EnableSSH)
		sys.POST("/ssh/disable", h.DisableSSH)
		sys.POST("/ssh/autostart", h.SetSSHAutoStart)
		// 远程访问安全设置
		sys.GET("/security/remote-access", h.GetRemoteAccessSettings)
		sys.PUT("/security/remote-access", h.UpdateRemoteAccessSettings)
		// 代理设置
		sys.GET("/proxy", h.GetProxyConfig)
		sys.POST("/proxy", h.SaveProxyConfig)
		sys.POST("/proxy/test", h.TestProxy)
		// i18n 国际化设置
		sys.GET("/i18n", h.GetI18nSettings)
		sys.PUT("/i18n", h.UpdateI18nSettings)
		sys.GET("/i18n/options", h.GetI18nOptions)
		sys.GET("/i18n/detect", h.DetectRegion)
		sys.POST("/i18n/preview-switch", h.PreviewRegionSwitch)
	}
}
