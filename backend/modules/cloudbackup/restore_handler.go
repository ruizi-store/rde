package cloudbackup

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CloudRestoreHandler 云端恢复处理器
type CloudRestoreHandler struct {
	db      *gorm.DB
	dataDir string
	logger  *zap.Logger

	// 恢复进度跟踪
	mu       sync.RWMutex
	progress *RestoreProgress
}

// RestoreProgress 恢复进度
type RestoreProgress struct {
	Stage   string `json:"stage"`   // connecting, downloading, decrypting, extracting, importing, restoring_files, completed, error
	Percent int    `json:"percent"` // 0-100
	Error   string `json:"error,omitempty"`
}

// NewCloudRestoreHandler 创建云端恢复处理器
func NewCloudRestoreHandler(db *gorm.DB, dataDir string, logger *zap.Logger) *CloudRestoreHandler {
	return &CloudRestoreHandler{
		db:      db,
		dataDir: dataDir,
		logger:  logger,
	}
}

// CloudLoginRequest 云端登录请求
type CloudLoginRequest struct {
	Email string `json:"email" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// CloudLoginResponse 云端登录响应
type CloudLoginResponse struct {
	Token string `json:"token"`
}

// CloudLogin 通过邮箱验证码登录获取 cloud token
// POST /api/v1/setup/cloud/login
func (h *CloudRestoreHandler) CloudLogin(c *gin.Context) {
	var req CloudLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱和验证码"})
		return
	}

	// 调用 rde-cloud API 验证并获取 token
	cloudURL := "https://rde.lidj.cn"
	client := &http.Client{}

	// 构造请求
	loginURL := cloudURL + "/api/v1/auth/email/verify"
	body := `{"email":"` + req.Email + `","code":"` + req.Code + `","purpose":"login"}`
	httpReq, _ := http.NewRequest("POST", loginURL, nil)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Body = io.NopCloser(strings.NewReader(body))

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法连接到云服务"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "验证失败"})
		return
	}

	// 解析返回的 token
	var result struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析响应失败"})
		return
	}

	c.JSON(http.StatusOK, CloudLoginResponse{Token: result.Token.AccessToken})
}

// CloudSendEmail 发送邮箱验证码
// POST /api/v1/setup/cloud/email
func (h *CloudRestoreHandler) CloudSendEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入邮箱"})
		return
	}

	cloudURL := "https://rde.lidj.cn"
	client := &http.Client{}

	emailURL := cloudURL + "/api/v1/email/send"
	body := `{"email":"` + req.Email + `","purpose":"login"}`
	httpReq, _ := http.NewRequest("POST", emailURL, nil)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Body = io.NopCloser(strings.NewReader(body))

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法连接到云服务"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "发送验证码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送"})
}

// ListBackupsRequest 列出备份请求
type ListBackupsRequest struct {
	Token string `json:"token" binding:"required"`
}

// CloudListBackups 列出云端备份
// POST /api/v1/setup/cloud/backups
func (h *CloudRestoreHandler) CloudListBackups(c *gin.Context) {
	var req ListBackupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 token"})
		return
	}

	cloudURL := "https://rde.lidj.cn"
	restoreService := NewCloudRestoreService(h.db, h.dataDir, h.logger)
	items, err := restoreService.ListCloudBackups(req.Token, cloudURL)
	if err != nil {
		h.logger.Error("列出云端备份失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取备份列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"backups": items})
}

// CloudRestoreRequest 恢复请求
type CloudRestoreRequest struct {
	Token    string `json:"token" binding:"required"`
	BackupID string `json:"backup_id" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CloudRestore 执行云端恢复
// POST /api/v1/setup/cloud/restore
func (h *CloudRestoreHandler) CloudRestore(c *gin.Context) {
	var req CloudRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	// 检查是否已有恢复在进行
	h.mu.RLock()
	if h.progress != nil && h.progress.Stage != "completed" && h.progress.Stage != "error" {
		h.mu.RUnlock()
		c.JSON(http.StatusConflict, gin.H{"error": "恢复正在进行中"})
		return
	}
	h.mu.RUnlock()

	// 初始化进度
	h.mu.Lock()
	h.progress = &RestoreProgress{Stage: "starting", Percent: 0}
	h.mu.Unlock()

	cloudURL := "https://rde.lidj.cn"
	restoreService := NewCloudRestoreService(h.db, h.dataDir, h.logger)

	// 异步执行恢复
	go func() {
		err := restoreService.RestoreFromCloud(req.Token, cloudURL, req.BackupID, req.Password, func(stage string, pct int) {
			h.mu.Lock()
			h.progress = &RestoreProgress{Stage: stage, Percent: pct}
			h.mu.Unlock()
		})

		h.mu.Lock()
		if err != nil {
			h.progress = &RestoreProgress{Stage: "error", Percent: 0, Error: err.Error()}
			h.logger.Error("云端恢复失败", zap.Error(err))
		} else {
			h.progress = &RestoreProgress{Stage: "completed", Percent: 100}
			h.logger.Info("云端恢复完成")
		}
		h.mu.Unlock()
	}()

	c.JSON(http.StatusOK, gin.H{"message": "恢复已开始"})
}

// CloudRestoreStatus 查询恢复进度
// GET /api/v1/setup/cloud/restore/status
func (h *CloudRestoreHandler) CloudRestoreStatus(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.progress == nil {
		c.JSON(http.StatusOK, RestoreProgress{Stage: "idle", Percent: 0})
		return
	}

	c.JSON(http.StatusOK, h.progress)
}
