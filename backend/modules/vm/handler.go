// Package vm HTTP 处理器
package vm

import (
	"io"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
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
	vm := r.Group("/vm")
	{
		vm.GET("/vms", h.GetVMs)
		vm.POST("/vms", h.CreateVM)
		vm.GET("/vms/:id", h.GetVM)
		vm.PUT("/vms/:id", h.UpdateVM)
		vm.DELETE("/vms/:id", h.DeleteVM)

		vm.POST("/vms/:id/start", h.StartVM)
		vm.POST("/vms/:id/stop", h.StopVM)
		vm.POST("/vms/:id/restart", h.RestartVM)
		vm.POST("/vms/:id/pause", h.PauseVM)
		vm.POST("/vms/:id/resume", h.ResumeVM)

		// 批量操作
		vm.POST("/batch/start", h.BatchStart)
		vm.POST("/batch/stop", h.BatchStop)
		vm.POST("/batch/delete", h.BatchDelete)

		// 克隆
		vm.POST("/vms/:id/clone", h.CloneVM)

		vm.GET("/vms/:id/vnc", h.GetVNC)
		vm.POST("/vms/:id/vnc/token", h.GetVNCToken)
		vm.GET("/vnc/websocket", h.VNCWebSocket)
		vm.POST("/vms/:id/resize", h.ResizeDisk)

		// QMP 操作
		vm.POST("/vms/:id/sendkey", h.SendKey)
		vm.POST("/vms/:id/ctrlaltdel", h.SendCtrlAltDel)
		vm.POST("/vms/:id/screendump", h.Screendump)

		vm.GET("/vms/:id/stats", h.GetVMStats)
		vm.GET("/stats", h.GetAllVMStats)
		vm.GET("/storage", h.GetStorageInfo)

		// 硬件设备
		vm.GET("/usb-devices", h.ListUSBDevices)
		vm.GET("/network-interfaces", h.ListNetworkInterfaces)

		vm.GET("/vms/:id/snapshots", h.GetSnapshots)
		vm.POST("/vms/:id/snapshots", h.CreateSnapshot)
		vm.DELETE("/vms/:id/snapshots/:tag", h.DeleteSnapshot)
		vm.POST("/vms/:id/snapshots/:tag/revert", h.RevertSnapshot)

		// 模板管理
		vm.GET("/templates", h.GetTemplates)
		vm.POST("/templates", h.CreateTemplate)
		vm.DELETE("/templates/:id", h.DeleteTemplate)

		// 备份管理
		vm.GET("/backups", h.ListBackups)
		vm.POST("/backups", h.CreateBackup)
		vm.POST("/backups/:id/restore", h.RestoreBackup)
		vm.DELETE("/backups/:id", h.DeleteBackup)

		vm.GET("/isos", h.GetISOs)
		vm.POST("/isos", h.UploadISO)
		vm.DELETE("/isos/:name", h.DeleteISO)

		// P5: 自动启动
		vm.PUT("/vms/:id/autostart", h.SetAutoStart)
		vm.GET("/autostart", h.GetAutoStartVMs)
		vm.POST("/autostart/run", h.RunAutoStart)

		// P5: 导入导出
		vm.POST("/export", h.ExportVM)
		vm.POST("/import", h.ImportVM)

		// P5: 实时快照
		vm.POST("/vms/:id/live-snapshot", h.CreateLiveSnapshot)

		// P6: SSH 终端集成
		vm.GET("/vms/:id/ssh-info", h.GetVMSSHInfo)

		// P6: 资源监控
		vm.GET("/vms/:id/resources", h.GetVMResources)
		vm.GET("/resources", h.GetAllVMResources)
	}
}

// GetVMs 获取虚拟机列表
func (h *Handler) GetVMs(c *gin.Context) {
	vms := h.service.GetVMs()
	c.JSON(http.StatusOK, vms)
}

// GetVM 获取虚拟机
func (h *Handler) GetVM(c *gin.Context) {
	id := c.Param("id")
	vm, err := h.service.GetVM(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vm)
}

// CreateVM 创建虚拟机
func (h *Handler) CreateVM(c *gin.Context) {
	var req CreateVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.service.CreateVM(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vm)
}

// UpdateVM 更新虚拟机
func (h *Handler) UpdateVM(c *gin.Context) {
	id := c.Param("id")
	var req UpdateVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.service.UpdateVM(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vm)
}

// DeleteVM 删除虚拟机
func (h *Handler) DeleteVM(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteVM(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "vm deleted"})
}

// StartVM 启动虚拟机
func (h *Handler) StartVM(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.StartVM(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "vm started"})
}

// StopVM 停止虚拟机
func (h *Handler) StopVM(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"
	if err := h.service.StopVM(id, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "vm stopped"})
}

// PauseVM 暂停虚拟机
func (h *Handler) PauseVM(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.PauseVM(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "vm paused"})
}

// ResumeVM 恢复虚拟机
func (h *Handler) ResumeVM(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.ResumeVM(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "vm resumed"})
}

// GetVNC 获取 VNC 连接信息
func (h *Handler) GetVNC(c *gin.Context) {
	id := c.Param("id")
	addr, err := h.service.GetVNCWebSocket(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"address": addr})
}

// ResizeDisk 调整磁盘大小
func (h *Handler) ResizeDisk(c *gin.Context) {
	id := c.Param("id")
	var req ResizeDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ResizeDisk(id, req.Size); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "disk resized"})
}

// GetSnapshots 获取快照列表
func (h *Handler) GetSnapshots(c *gin.Context) {
	id := c.Param("id")
	snapshots, err := h.service.GetSnapshots(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshots)
}

// CreateSnapshot 创建快照
func (h *Handler) CreateSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req CreateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snapshot, err := h.service.CreateSnapshot(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

// DeleteSnapshot 删除快照
func (h *Handler) DeleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	tag := c.Param("tag")
	if err := h.service.DeleteSnapshot(id, tag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "snapshot deleted"})
}

// RevertSnapshot 恢复快照
func (h *Handler) RevertSnapshot(c *gin.Context) {
	id := c.Param("id")
	tag := c.Param("tag")
	if err := h.service.RevertSnapshot(id, tag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "snapshot reverted"})
}

// GetTemplates 获取模板
func (h *Handler) GetTemplates(c *gin.Context) {
	templates := h.service.GetTemplates()
	c.JSON(http.StatusOK, templates)
}

// GetISOs 获取 ISO 列表
func (h *Handler) GetISOs(c *gin.Context) {
	isos, err := h.service.GetISOs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, isos)
}

// RestartVM 重启虚拟机
func (h *Handler) RestartVM(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.RestartVM(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "vm restarted"})
}

// GetVMStats 获取虚拟机资源统计
func (h *Handler) GetVMStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.service.GetVMStats(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetAllVMStats 获取所有运行中 VM 的统计
func (h *Handler) GetAllVMStats(c *gin.Context) {
	stats := h.service.GetAllVMStats()
	c.JSON(http.StatusOK, stats)
}

// UploadISO 上传 ISO 文件
func (h *Handler) UploadISO(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded"})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext != ".iso" && ext != ".ISO" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only ISO files are allowed"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	path, err := h.service.UploadISO(header.Filename, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "ISO uploaded", "path": path})
}

// DeleteISO 删除 ISO 文件
func (h *Handler) DeleteISO(c *gin.Context) {
	name := c.Param("name")
	if err := h.service.DeleteISO(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ISO deleted"})
}

// GetVNCToken 获取 VNC 连接令牌
func (h *Handler) GetVNCToken(c *gin.Context) {
	id := c.Param("id")
	token, err := h.service.GetVNCToken(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// VNCWebSocket VNC WebSocket 代理
func (h *Handler) VNCWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	vncToken, valid := h.service.ValidateVNCToken(token)
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	h.service.GetVNCProxy().HandleWebSocket(c, "127.0.0.1", vncToken.VNCPort)
}

// SendKey 发送按键到虚拟机
func (h *Handler) SendKey(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Keys []string `json:"keys" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SendKey(id, req.Keys); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "key sent"})
}

// SendCtrlAltDel 发送 Ctrl+Alt+Del
func (h *Handler) SendCtrlAltDel(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.SendCtrlAltDel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ctrl+alt+del sent"})
}

// Screendump 截取屏幕
func (h *Handler) Screendump(c *gin.Context) {
	id := c.Param("id")
	path, err := h.service.Screendump(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path})
}

// ==================== P2: 模板管理 ====================

// CreateTemplate 从现有VM创建模板
func (h *Handler) CreateTemplate(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, err := h.service.CreateTemplateFromVM(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, template)
}

// DeleteTemplate 删除自定义模板
func (h *Handler) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "template deleted"})
}

// ==================== P2: 备份管理 ====================

// CreateBackup 创建VM备份
func (h *Handler) CreateBackup(c *gin.Context) {
	var req CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	backup, err := h.service.CreateBackup(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, backup)
}

// ListBackups 列出备份
func (h *Handler) ListBackups(c *gin.Context) {
	vmID := c.Query("vm_id")
	backups, err := h.service.ListBackups(vmID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, backups)
}

// RestoreBackup 从备份恢复VM
func (h *Handler) RestoreBackup(c *gin.Context) {
	backupID := c.Param("id")
	var req struct {
		NewName string `json:"new_name,omitempty"`
	}
	c.ShouldBindJSON(&req)

	restoreReq := RestoreBackupRequest{
		BackupID: backupID,
		NewName:  req.NewName,
	}

	vm, err := h.service.RestoreBackup(restoreReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vm)
}

// DeleteBackup 删除备份
func (h *Handler) DeleteBackup(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteBackup(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "backup deleted"})
}

// ==================== P3: 批量操作 ====================

// BatchStart 批量启动虚拟机
func (h *Handler) BatchStart(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := h.service.BatchStart(req.IDs)
	c.JSON(http.StatusOK, results)
}

// BatchStop 批量停止虚拟机
func (h *Handler) BatchStop(c *gin.Context) {
	var req struct {
		IDs   []string `json:"ids" binding:"required"`
		Force bool     `json:"force,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := h.service.BatchStop(req.IDs, req.Force)
	c.JSON(http.StatusOK, results)
}

// BatchDelete 批量删除虚拟机
func (h *Handler) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := h.service.BatchDelete(req.IDs)
	c.JSON(http.StatusOK, results)
}

// ==================== P3: 克隆 ====================

// CloneVM 克隆虚拟机
func (h *Handler) CloneVM(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.service.CloneVM(id, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vm)
}

// ==================== P3: 存储信息 ====================

// GetStorageInfo 获取存储使用信息
func (h *Handler) GetStorageInfo(c *gin.Context) {
	info, err := h.service.GetStorageInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

// ==================== P4: 硬件设备 ====================

// ListUSBDevices 列出主机 USB 设备
func (h *Handler) ListUSBDevices(c *gin.Context) {
	devices, err := h.service.ListUSBDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

// ListNetworkInterfaces 列出可用网络接口
func (h *Handler) ListNetworkInterfaces(c *gin.Context) {
	interfaces, err := h.service.ListNetworkInterfaces()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, interfaces)
}

// ==================== P5: 自动启动 ====================

// SetAutoStart 设置自动启动
func (h *Handler) SetAutoStart(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enabled bool `json:"enabled"`
		Order   int  `json:"order"`
		Delay   int  `json:"delay"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SetAutoStart(id, req.Enabled, req.Order, req.Delay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GetAutoStartVMs 获取自动启动 VM 列表
func (h *Handler) GetAutoStartVMs(c *gin.Context) {
	vms := h.service.GetAutoStartVMs()
	c.JSON(http.StatusOK, vms)
}

// RunAutoStart 执行自动启动
func (h *Handler) RunAutoStart(c *gin.Context) {
	results := h.service.StartAutoStartVMs()
	c.JSON(http.StatusOK, results)
}

// ==================== P5: 导入导出 ====================

// ExportVM 导出虚拟机
func (h *Handler) ExportVM(c *gin.Context) {
	var req ExportVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.service.ExportVM(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ImportVM 导入虚拟机
func (h *Handler) ImportVM(c *gin.Context) {
	var req ImportVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.service.ImportVM(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vm)
}

// ==================== P5: 实时快照 ====================

// CreateLiveSnapshot 创建实时快照
func (h *Handler) CreateLiveSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req LiveSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snapshot, err := h.service.CreateLiveSnapshot(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

// ==================== P6: SSH 终端集成 ====================

// GetVMSSHInfo 获取 VM SSH 连接信息
func (h *Handler) GetVMSSHInfo(c *gin.Context) {
	id := c.Param("id")

	info, err := h.service.GetVMSSHInfo(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// ==================== P6: 资源监控 ====================

// GetVMResources 获取单个 VM 资源统计
func (h *Handler) GetVMResources(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.service.GetVMResourceStats(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetAllVMResources 获取所有运行中 VM 的资源统计
func (h *Handler) GetAllVMResources(c *gin.Context) {
	stats, err := h.service.GetAllVMsStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
