package backup

import (
	"io"
	"net/http"

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
	backup := r.Group("/backup")
	{
		// 概览
		backup.GET("/overview", h.GetOverview)

		// 任务管理
		backup.GET("/tasks", h.ListTasks)
		backup.POST("/tasks", h.CreateTask)
		backup.GET("/tasks/:id", h.GetTask)
		backup.PUT("/tasks/:id", h.UpdateTask)
		backup.DELETE("/tasks/:id", h.DeleteTask)
		backup.POST("/tasks/:id/run", h.RunTask)

		// 备份记录
		backup.GET("/records", h.ListRecords)
		backup.GET("/records/:id", h.GetRecord)
		backup.DELETE("/records/:id", h.DeleteRecord)

		// 还原
		backup.POST("/restore", h.Restore)
		backup.GET("/restore/:id/status", h.GetRestoreStatus)

		// 目标测试
		backup.POST("/targets/test", h.TestTarget)

		// 配置导入导出
		backup.GET("/config/exportable", h.GetExportableConfigs)
		backup.POST("/config/export", h.ExportConfig)
		backup.POST("/config/import", h.ImportConfig)
	}
}

// GetOverview 获取概览
func (h *Handler) GetOverview(c *gin.Context) {
	overview, err := h.service.GetOverview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": overview})
}

// ListTasks 获取任务列表
func (h *Handler) ListTasks(c *gin.Context) {
	var req ListTasksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	tasks, total, err := h.service.ListTasks(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
		"total":   total,
	})
}

// CreateTask 创建任务
func (h *Handler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	task, err := h.service.CreateTask(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// GetTask 获取任务详情
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.service.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// UpdateTask 更新任务
func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	task, err := h.service.UpdateTask(id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// DeleteTask 删除任务
func (h *Handler) DeleteTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RunTask 立即执行任务
func (h *Handler) RunTask(c *gin.Context) {
	id := c.Param("id")

	record, err := h.service.RunTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": record})
}

// ListRecords 获取备份记录
func (h *Handler) ListRecords(c *gin.Context) {
	var req ListRecordsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	records, total, err := h.service.ListRecords(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    records,
		"total":   total,
	})
}

// GetRecord 获取记录详情
func (h *Handler) GetRecord(c *gin.Context) {
	id := c.Param("id")

	record, err := h.service.GetRecord(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "记录不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": record})
}

// DeleteRecord 删除记录
func (h *Handler) DeleteRecord(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteRecord(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Restore 执行还原
func (h *Handler) Restore(c *gin.Context) {
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	status, err := h.service.Restore(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

// GetRestoreStatus 获取还原状态
func (h *Handler) GetRestoreStatus(c *gin.Context) {
	id := c.Param("id")

	status, err := h.service.GetRestoreStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "记录不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

// TestTarget 测试目标连接
func (h *Handler) TestTarget(c *gin.Context) {
	var req TargetTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	result := h.service.TestTarget(&req)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// GetExportableConfigs 获取可导出配置
func (h *Handler) GetExportableConfigs(c *gin.Context) {
	configs := h.service.GetExportableConfigs()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": configs})
}

// ExportConfig 导出配置
func (h *Handler) ExportConfig(c *gin.Context) {
	var req ConfigExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	data, err := h.service.ExportConfig(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=rde-config.json")
	c.Data(http.StatusOK, "application/json", data)
}

// ImportConfig 导入配置
func (h *Handler) ImportConfig(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请上传配置文件"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	var req ConfigImportRequest
	req.Password = c.PostForm("password")
	req.Overwrite = c.PostForm("overwrite") == "true"

	if err := h.service.ImportConfig(data, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置导入成功"})
}
