// Package ai HTTP 处理器
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler HTTP 处理器
type Handler struct {
	service  *Service
	skills   *SkillsService
	setup    *SetupService
	gateway  *GatewayService
	alerts   *AlertService
	sessions *SessionStore
	voice    *VoiceService
}

// NewHandler 创建处理器实例
func NewHandler(service *Service, skills *SkillsService, setup *SetupService, gateway *GatewayService, alerts *AlertService, sessions *SessionStore, voice *VoiceService) *Handler {
	return &Handler{service: service, skills: skills, setup: setup, gateway: gateway, alerts: alerts, sessions: sessions, voice: voice}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	ai := r.Group("/ai")
	{
		// 提供商
		ai.GET("/providers", h.GetProviders)
		ai.POST("/providers", h.CreateProvider)
		ai.GET("/providers/:id", h.GetProvider)
		ai.PUT("/providers/:id", h.UpdateProvider)
		ai.DELETE("/providers/:id", h.DeleteProvider)

		// 模型
		ai.GET("/providers/:id/models", h.GetModels)
		ai.POST("/providers/:id/models/pull", h.PullModel)
		ai.DELETE("/providers/:id/models", h.DeleteModel)

		// 聊天
		ai.POST("/chat", h.Chat)
		ai.POST("/chat/stream", h.ChatStream)

		// 对话
		ai.GET("/conversations", h.GetConversations)
		ai.GET("/conversations/search", h.SearchConversations)
		ai.POST("/conversations", h.CreateConversation)
		ai.GET("/conversations/:id", h.GetConversation)
		ai.PUT("/conversations/:id", h.UpdateConversation)
		ai.DELETE("/conversations/:id", h.DeleteConversation)
		ai.DELETE("/conversations/:id/messages", h.ClearMessages)
		ai.POST("/conversations/:id/messages/save", h.SaveMessages)

		// 配置/状态
		ai.GET("/config", h.GetConfig)
		ai.PUT("/config", h.UpdateConfig)
		ai.GET("/status", h.GetStatus)

		// 向导
		ai.GET("/setup/status", h.GetSetupStatus)
		ai.POST("/setup/check-env", h.CheckEnvironment)
		ai.GET("/setup/models", h.GetAvailableModels)
		ai.POST("/setup/models/download", h.DownloadModel)
		ai.GET("/setup/skills", h.GetDefaultSkills)
		ai.PUT("/setup/step", h.SetSetupStep)
		ai.POST("/setup/complete", h.CompleteSetup)
		ai.POST("/setup/reset", h.ResetSetup)

		// 技能
		ai.POST("/skills/execute", h.ExecuteSkill)
		ai.GET("/storage/analysis", h.GetStorageAnalysis)
		ai.GET("/system/info", h.GetSystemInfo)
		ai.GET("/files/search", h.SearchFiles)

		// 网关
		ai.GET("/gateway/config", h.GetGatewayConfig)
		ai.PUT("/gateway/config", h.UpdateGatewayConfig)
		ai.GET("/gateway/status", h.GetGatewayStatus)
		ai.POST("/gateway/start", h.StartGateway)
		ai.POST("/gateway/stop", h.StopGateway)
		ai.POST("/gateway/test-telegram", h.TestTelegram)
		ai.POST("/gateway/wecom/callback", h.WecomCallback)
		ai.POST("/gateway/telegram/webhook", h.TelegramWebhook)
		ai.POST("/gateway/webhook/message", h.WebhookMessage)

		// 告警
		ai.GET("/alerts/config", h.GetAlertsConfig)
		ai.PUT("/alerts/config", h.UpdateAlertsConfig)
		ai.GET("/alerts/status", h.GetAlertsStatus)
		ai.GET("/alerts/history", h.GetAlertsHistory)
		ai.DELETE("/alerts/history", h.ClearAlertsHistory)
		ai.POST("/alerts/start", h.StartAlerts)
		ai.POST("/alerts/stop", h.StopAlerts)

		// 会话
		ai.GET("/sessions/stats", h.GetSessionStats)
		ai.GET("/sessions", h.GetSessions)
		ai.GET("/sessions/:userId/messages", h.GetUserMessages)
		ai.GET("/sessions/:userId/export", h.ExportUserData)
		ai.DELETE("/sessions/:userId", h.DeleteUserData)

		// 语音
		ai.GET("/voice/config", h.GetVoiceConfig)
		ai.PUT("/voice/config", h.UpdateVoiceConfig)
		ai.POST("/voice/transcribe", h.TranscribeAudio)
		ai.POST("/voice/tts", h.TextToSpeech)
	}
}

// GetProviders 获取提供商列表
func (h *Handler) GetProviders(c *gin.Context) {
	providers := h.service.GetProviders()
	c.JSON(http.StatusOK, providers)
}

// GetProvider 获取提供商
func (h *Handler) GetProvider(c *gin.Context) {
	id := c.Param("id")
	provider, err := h.service.GetProvider(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// CreateProvider 创建提供商
func (h *Handler) CreateProvider(c *gin.Context) {
	var req CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider, err := h.service.CreateProvider(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// UpdateProvider 更新提供商
func (h *Handler) UpdateProvider(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider, err := h.service.UpdateProvider(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// DeleteProvider 删除提供商
func (h *Handler) DeleteProvider(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteProvider(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "provider deleted"})
}

// GetModels 获取模型列表
func (h *Handler) GetModels(c *gin.Context) {
	id := c.Param("id")
	models, err := h.service.GetModels(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, models)
}

// PullModel 拉取模型 (Ollama)
func (h *Handler) PullModel(c *gin.Context) {
	id := c.Param("id")
	var req OllamaPullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.PullOllamaModel(id, req.Model); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "model pulled"})
}

// DeleteModel 删除模型 (Ollama)
func (h *Handler) DeleteModel(c *gin.Context) {
	id := c.Param("id")
	var req OllamaDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.DeleteOllamaModel(id, req.Model); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "model deleted"})
}

// Chat 聊天
func (h *Handler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.Chat(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存消息到对话（只保存新消息，避免重复）
	if req.ConversationID != "" {
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			if lastMsg.Role == "user" {
				h.service.AddMessage(req.ConversationID, lastMsg)
			}
		}
		h.service.AddMessage(req.ConversationID, Message{
			Role:    "assistant",
			Content: resp.Content,
		})
	}

	c.JSON(http.StatusOK, resp)
}

// ChatStream 流式聊天
func (h *Handler) ChatStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Stream = true
	stream := make(chan StreamChunk, 100)

	// 启动流式请求
	streamErr := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				streamErr <- fmt.Errorf("stream panic: %v", r)
			}
		}()
		streamErr <- h.service.ChatStream(req, stream)
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	var fullContent string
	c.Stream(func(w io.Writer) bool {
		if chunk, ok := <-stream; ok {
			fullContent += chunk.Delta
			data, _ := json.Marshal(chunk)
			w.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return true
		}
		// channel 关闭后检查是否有错误
		select {
		case err := <-streamErr:
			if err != nil {
				errChunk := StreamChunk{Error: err.Error()}
				data, _ := json.Marshal(errChunk)
				w.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		default:
		}
		return false
	})

	// 保存消息到对话（只保存新消息，避免重复）
	if req.ConversationID != "" && fullContent != "" {
		// 只保存最后一条用户消息和助手回复
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			if lastMsg.Role == "user" {
				h.service.AddMessage(req.ConversationID, lastMsg)
			}
		}
		h.service.AddMessage(req.ConversationID, Message{
			Role:    "assistant",
			Content: fullContent,
		})
	}
}

// GetConversations 获取对话列表
func (h *Handler) GetConversations(c *gin.Context) {
	convs := h.service.GetConversations()
	c.JSON(http.StatusOK, convs)
}

// SearchConversations 搜索对话（GET /ai/conversations/search?q=...)
func (h *Handler) SearchConversations(c *gin.Context) {
	query := c.Query("q")
	convs := h.service.SearchConversations(query)
	c.JSON(http.StatusOK, convs)
}

// GetConversation 获取对话
func (h *Handler) GetConversation(c *gin.Context) {
	id := c.Param("id")
	conv, err := h.service.GetConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

// CreateConversation 创建对话
func (h *Handler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := h.service.CreateConversation(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

// UpdateConversation 更新对话
func (h *Handler) UpdateConversation(c *gin.Context) {
	id := c.Param("id")
	var req UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := h.service.UpdateConversation(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

// DeleteConversation 删除对话
func (h *Handler) DeleteConversation(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteConversation(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted"})
}

// ClearMessages 清空对话消息（保留对话本身）
func (h *Handler) ClearMessages(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.ClearMessages(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "messages cleared"})
}

// SaveMessages 批量保存消息
func (h *Handler) SaveMessages(c *gin.Context) {
	id := c.Param("id")
	var req SaveMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.SaveMessages(id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "messages saved"})
}

// GetConfig 获取 AI 配置
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.service.GetAIConfig())
}

// UpdateConfig 更新 AI 配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req AIConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.UpdateAIConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, h.service.GetAIConfig())
}

// GetStatus 获取 AI 状态
func (h *Handler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.service.GetAIStatus())
}

// === 向导 ===

// GetSetupStatus 获取向导状态
func (h *Handler) GetSetupStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.setup.GetState())
}

// CheckEnvironment 检查环境
func (h *Handler) CheckEnvironment(c *gin.Context) {
	check := h.setup.CheckEnvironment()
	c.JSON(http.StatusOK, check)
}

// GetAvailableModels 获取可选模型
func (h *Handler) GetAvailableModels(c *gin.Context) {
	c.JSON(http.StatusOK, h.setup.GetAvailableModels())
}

// DownloadModel 下载模型（SSE 流式进度）
func (h *Handler) DownloadModel(c *gin.Context) {
	var req DownloadModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	progressCh := make(chan DownloadProgress, 10)
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		defer close(progressCh)
		h.setup.DownloadModel(ctx, req.Model, progressCh)
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		if p, ok := <-progressCh; ok {
			data, _ := json.Marshal(p)
			w.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return true
		}
		return false
	})
}

// GetDefaultSkills 获取默认技能
func (h *Handler) GetDefaultSkills(c *gin.Context) {
	c.JSON(http.StatusOK, h.setup.GetDefaultSkills())
}

// SetSetupStep 设置向导步骤
func (h *Handler) SetSetupStep(c *gin.Context) {
	var req struct {
		Step SetupStep `json:"step"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.setup.SetStep(req.Step)
	c.JSON(http.StatusOK, gin.H{"step": req.Step})
}

// CompleteSetup 完成向导
func (h *Handler) CompleteSetup(c *gin.Context) {
	var req SetupCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.setup.Complete(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "setup completed"})
}

// ResetSetup 重置向导
func (h *Handler) ResetSetup(c *gin.Context) {
	h.setup.Reset()
	c.JSON(http.StatusOK, gin.H{"message": "setup reset"})
}

// === 技能 ===

// ExecuteSkill 执行技能
func (h *Handler) ExecuteSkill(c *gin.Context) {
	var req SkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := h.skills.ExecuteSkill(&req)
	c.JSON(http.StatusOK, result)
}

// GetStorageAnalysis 获取存储分析
func (h *Handler) GetStorageAnalysis(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	analysis, err := h.skills.GetStorageAnalysis(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, SkillResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SkillResponse{Success: true, Data: analysis})
}

// GetSystemInfo 获取系统信息
func (h *Handler) GetSystemInfo(c *gin.Context) {
	info, err := h.skills.GetSystemInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, SkillResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SkillResponse{Success: true, Data: info})
}

// SearchFiles 搜索文件
func (h *Handler) SearchFiles(c *gin.Context) {
	query := c.Query("q")
	dir := c.DefaultQuery("dir", "/")
	results, err := h.skills.SearchFiles(dir, query, "", 0, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, SkillResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SkillResponse{Success: true, Data: results})
}

// === 网关 ===

// GetGatewayConfig 获取网关配置
func (h *Handler) GetGatewayConfig(c *gin.Context) {
	config := h.gateway.GetConfig()
	// 隐藏敏感信息
	safe := *config
	if safe.Telegram.BotToken != "" {
		safe.Telegram.BotToken = "***"
	}
	if safe.Wecom.Secret != "" {
		safe.Wecom.Secret = "***"
	}
	c.JSON(http.StatusOK, safe)
}

// UpdateGatewayConfig 更新网关配置
func (h *Handler) UpdateGatewayConfig(c *gin.Context) {
	var config GatewayConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.gateway.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// GetGatewayStatus 获取网关状态
func (h *Handler) GetGatewayStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.gateway.GetStatus())
}

// StartGateway 启动网关
func (h *Handler) StartGateway(c *gin.Context) {
	if err := h.gateway.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "gateway started"})
}

// StopGateway 停止网关
func (h *Handler) StopGateway(c *gin.Context) {
	h.gateway.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "gateway stopped"})
}

// TestTelegram 测试 Telegram 连接
func (h *Handler) TestTelegram(c *gin.Context) {
	adapter := h.gateway.GetAdapter(PlatformTelegram)
	if adapter == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "telegram adapter not configured"})
		return
	}
	ta, ok := adapter.(*TelegramAdapter)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid adapter"})
		return
	}
	if err := ta.TestConnection(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "connection successful"})
}

// WecomCallback 企业微信回调
func (h *Handler) WecomCallback(c *gin.Context) {
	adapter := h.gateway.GetAdapter(PlatformWecom)
	if adapter == nil {
		c.String(http.StatusOK, "")
		return
	}
	wa, ok := adapter.(*WecomAdapter)
	if !ok {
		c.String(http.StatusOK, "")
		return
	}
	body, _ := io.ReadAll(c.Request.Body)
	result, err := wa.HandleCallback(
		c.Query("msg_signature"), c.Query("timestamp"), c.Query("nonce"), c.Query("echostr"), body,
	)
	if err != nil {
		c.String(http.StatusOK, "")
		return
	}
	c.String(http.StatusOK, result)
}

// TelegramWebhook Telegram Webhook
func (h *Handler) TelegramWebhook(c *gin.Context) {
	adapter := h.gateway.GetAdapter(PlatformTelegram)
	if adapter == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	ta, ok := adapter.(*TelegramAdapter)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	body, _ := io.ReadAll(c.Request.Body)
	ta.HandleWebhook(body)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// WebhookMessage 通用 Webhook 消息
func (h *Handler) WebhookMessage(c *gin.Context) {
	adapter := h.gateway.GetAdapter(PlatformWebhook)
	if adapter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook not configured"})
		return
	}
	wa, ok := adapter.(*WebhookAdapter)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid adapter"})
		return
	}
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp := wa.HandleRequest(c.GetHeader("Authorization"), req)
	if resp.Success {
		c.JSON(http.StatusOK, resp)
	} else {
		c.JSON(http.StatusBadRequest, resp)
	}
}

// === 告警 ===

// GetAlertsConfig 获取告警配置
func (h *Handler) GetAlertsConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.alerts.GetConfig())
}

// UpdateAlertsConfig 更新告警配置
func (h *Handler) UpdateAlertsConfig(c *gin.Context) {
	var config AlertConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.alerts.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// GetAlertsStatus 获取告警状态
func (h *Handler) GetAlertsStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.alerts.GetStatus())
}

// GetAlertsHistory 获取告警历史
func (h *Handler) GetAlertsHistory(c *gin.Context) {
	c.JSON(http.StatusOK, h.alerts.GetAlerts())
}

// ClearAlertsHistory 清除告警历史
func (h *Handler) ClearAlertsHistory(c *gin.Context) {
	h.alerts.ClearAlerts()
	c.JSON(http.StatusOK, gin.H{"message": "alerts cleared"})
}

// StartAlerts 启动告警
func (h *Handler) StartAlerts(c *gin.Context) {
	h.alerts.Start()
	c.JSON(http.StatusOK, gin.H{"message": "alerts started"})
}

// StopAlerts 停止告警
func (h *Handler) StopAlerts(c *gin.Context) {
	h.alerts.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "alerts stopped"})
}

// === 会话 ===

// GetSessionStats 获取会话统计
func (h *Handler) GetSessionStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.sessions.GetStats())
}

// GetSessions 获取所有会话
func (h *Handler) GetSessions(c *gin.Context) {
	c.JSON(http.StatusOK, h.sessions.GetAllSessions())
}

// GetUserMessages 获取用户消息
func (h *Handler) GetUserMessages(c *gin.Context) {
	userID := c.Param("userId")
	c.JSON(http.StatusOK, h.sessions.GetUserMessages(userID))
}

// ExportUserData 导出用户数据
func (h *Handler) ExportUserData(c *gin.Context) {
	userID := c.Param("userId")
	c.JSON(http.StatusOK, h.sessions.ExportUserData(userID))
}

// DeleteUserData 删除用户数据
func (h *Handler) DeleteUserData(c *gin.Context) {
	userID := c.Param("userId")
	deleted := h.sessions.DeleteUserData(userID)
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// === 语音 ===

// GetVoiceConfig 获取语音配置
func (h *Handler) GetVoiceConfig(c *gin.Context) {
	config := h.voice.GetConfig()
	// 隐藏敏感信息
	safe := *config
	if safe.OpenAIKey != "" {
		safe.OpenAIKey = "***"
	}
	if safe.AzureKey != "" {
		safe.AzureKey = "***"
	}
	c.JSON(http.StatusOK, safe)
}

// UpdateVoiceConfig 更新语音配置
func (h *Handler) UpdateVoiceConfig(c *gin.Context) {
	var config VoiceConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.voice.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "voice config updated"})
}

// TranscribeAudio 转录音频
func (h *Handler) TranscribeAudio(c *gin.Context) {
	file, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "audio file required"})
		return
	}

	// 使用随机文件名防止路径遍历
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".wav"
	}
	safeName := uuid.New().String() + ext
	tmpPath := filepath.Join(os.TempDir(), "voice_upload_"+safeName)
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	defer os.Remove(tmpPath)

	text, err := h.voice.TranscribeAudio(c.Request.Context(), tmpPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": text, "filename": file.Filename})
}

// TextToSpeech 文本转语音
func (h *Handler) TextToSpeech(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	audioPath, err := h.voice.TextToSpeech(c.Request.Context(), req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer os.Remove(audioPath)

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read audio"})
		return
	}
	c.Data(http.StatusOK, "audio/mpeg", audioData)
}
