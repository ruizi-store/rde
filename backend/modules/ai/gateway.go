// Package ai 消息网关核心服务
package ai

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PlatformAdapter 平台适配器接口
type PlatformAdapter interface {
	Platform() PlatformType
	Start(ctx context.Context) error
	Stop() error
	SendMessage(ctx context.Context, msg OutgoingMessage) error
	IsEnabled() bool
}

// GatewayService 消息网关服务
type GatewayService struct {
	logger     *zap.Logger
	dataDir    string
	config     *GatewayConfig
	configFile string

	adapters  map[PlatformType]PlatformAdapter
	sessions  map[string]*GatewaySession // key: platform:user_id
	sessionMu sync.RWMutex

	aiService *Service
	skills    *SkillsService

	// P3：按渠道解析绑定的 Bot（bot 模块注入）
	botResolver BotResolver

	// P4：审批管理（/approve /deny 命令路由）
	approvals ApprovalManager

	ctx    context.Context
	cancel context.CancelFunc
}

// setApprovalManager 设置网关的审批管理器
func (gs *GatewayService) setApprovalManager(am ApprovalManager) {
	gs.approvals = am
}

// NewGatewayService 创建网关服务
func NewGatewayService(logger *zap.Logger, dataDir string, aiService *Service, skills *SkillsService) *GatewayService {
	ctx, cancel := context.WithCancel(context.Background())

	gs := &GatewayService{
		logger:     logger,
		dataDir:    dataDir,
		configFile: filepath.Join(dataDir, "gateway.json"),
		adapters:   make(map[PlatformType]PlatformAdapter),
		sessions:   make(map[string]*GatewaySession),
		aiService:  aiService,
		skills:     skills,
		ctx:        ctx,
		cancel:     cancel,
	}

	gs.loadConfig()
	gs.initAdapters()
	return gs
}

func (gs *GatewayService) loadConfig() {
	gs.config = &GatewayConfig{
		Enabled:  false,
		Wecom:    WecomConfig{Enabled: false},
		Telegram: TelegramConfig{Enabled: false},
		Webhook: WebhookConfig{
			Enabled: false,
			APIKey:  generateAPIKey(),
		},
		Security: SecurityConfig{
			AllowedUsers:        []string{},
			RequireConfirmation: []string{"execute_script", "schedule_shutdown"},
			DailyLimit:          100,
			RequirePIN:          false,
		},
	}

	data, err := os.ReadFile(gs.configFile)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, gs.config); err != nil {
		gs.logger.Warn("failed to parse gateway config", zap.Error(err))
	}
}

func (gs *GatewayService) saveConfig() error {
	data, err := json.MarshalIndent(gs.config, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := gs.configFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpFile, gs.configFile)
}

// GetConfig 获取配置
func (gs *GatewayService) GetConfig() *GatewayConfig { return gs.config }

// GetTelegramWebhookSecret 读取 Telegram webhook secret（空表示未配置）
func (gs *GatewayService) GetTelegramWebhookSecret() string {
	if gs.config == nil {
		return ""
	}
	return gs.config.Telegram.WebhookSecret
}

// UpdateConfig 更新配置
func (gs *GatewayService) UpdateConfig(config *GatewayConfig) error {
	gs.config = config
	if err := gs.saveConfig(); err != nil {
		return err
	}
	gs.Stop()
	gs.initAdapters()
	if gs.config.Enabled {
		gs.Start()
	}
	return nil
}

func (gs *GatewayService) initAdapters() {
	gs.adapters = make(map[PlatformType]PlatformAdapter)

	if gs.config.Wecom.Enabled {
		gs.adapters[PlatformWecom] = NewWecomAdapter(gs.logger, &gs.config.Wecom, gs)
	}
	if gs.config.Telegram.Enabled {
		gs.adapters[PlatformTelegram] = NewTelegramAdapter(gs.logger, &gs.config.Telegram, gs)
	}
	gs.adapters[PlatformWebhook] = NewWebhookAdapter(gs.logger, &gs.config.Webhook, gs)
}

// Start 启动所有适配器
func (gs *GatewayService) Start() error {
	if !gs.config.Enabled {
		return nil
	}
	gs.ctx, gs.cancel = context.WithCancel(context.Background())

	for platform, adapter := range gs.adapters {
		if adapter.IsEnabled() {
			gs.logger.Info("Starting gateway adapter", zap.String("platform", string(platform)))
			go func(a PlatformAdapter) {
				if err := a.Start(gs.ctx); err != nil {
					gs.logger.Error("Adapter failed", zap.Error(err))
				}
			}(adapter)
		}
	}
	return nil
}

// Stop 停止所有适配器
func (gs *GatewayService) Stop() {
	if gs.cancel != nil {
		gs.cancel()
	}
	for platform, adapter := range gs.adapters {
		gs.logger.Info("Stopping gateway adapter", zap.String("platform", string(platform)))
		adapter.Stop()
	}
}

// HandleMessage 处理传入消息
func (gs *GatewayService) HandleMessage(ctx context.Context, msg IncomingMessage) (*OutgoingMessage, error) {
	gs.logger.Info("Received message",
		zap.String("platform", string(msg.Platform)),
		zap.String("user", msg.UserID),
		zap.String("text", msg.Text))

	if err := gs.checkSecurity(msg); err != nil {
		return &OutgoingMessage{
			Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: fmt.Sprintf("❌ 安全检查失败: %s", err.Error()),
		}, nil
	}

	session := gs.getOrCreateSession(msg)

	// P3：按渠道解析绑定的 Bot（Bot.Channels 匹配 platform + chat_id/user_id）
	bot := gs.resolveBotForSession(msg, session)

	// P4：审批命令（/approve|/deny <id>，只对绑定 Bot 的会话生效）
	if response := gs.handleApprovalCommand(msg, session); response != nil {
		return response, nil
	}

	if response := gs.handleSpecialCommand(msg, session); response != nil {
		return response, nil
	}

	session.Messages = append(session.Messages, GatewayMessage{Role: "user", Content: msg.Text})
	if len(session.Messages) > 20 {
		session.Messages = session.Messages[len(session.Messages)-20:]
	}

	response, err := gs.callAI(ctx, session, bot)
	if err != nil {
		return &OutgoingMessage{
			Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: fmt.Sprintf("❌ AI 调用失败: %s", err.Error()),
		}, nil
	}

	session.Messages = append(session.Messages, GatewayMessage{Role: "assistant", Content: response})

	return &OutgoingMessage{
		Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
		Text: response, ReplyToMsgID: msg.MessageID,
	}, nil
}

func (gs *GatewayService) checkSecurity(msg IncomingMessage) error {
	security := &gs.config.Security

	// 默认拒绝：未配置白名单时不允许任何外部消息触发工具调用
	if len(security.AllowedUsers) == 0 {
		return fmt.Errorf("网关未配置允许的用户白名单，已拒绝消息（请在网关设置中配置 AllowedUsers）")
	}

	userKey := fmt.Sprintf("%s:%s", msg.Platform, msg.UserID)
	allowed := false
	for _, u := range security.AllowedUsers {
		if u == userKey || u == msg.UserID {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("用户未授权")
	}

	if security.DailyLimit > 0 {
		session := gs.getOrCreateSession(msg)
		today := time.Now().Format("2006-01-02")
		if session.DailyDate != today {
			session.DailyDate = today
			session.DailyCount = 0
		}
		if session.DailyCount >= security.DailyLimit {
			return fmt.Errorf("已达每日请求上限 (%d)", security.DailyLimit)
		}
		session.DailyCount++
	}
	return nil
}

func (gs *GatewayService) getOrCreateSession(msg IncomingMessage) *GatewaySession {
	key := fmt.Sprintf("%s:%s", msg.Platform, msg.UserID)

	gs.sessionMu.Lock()
	defer gs.sessionMu.Unlock()

	if session, ok := gs.sessions[key]; ok {
		session.LastActive = time.Now()
		return session
	}

	session := &GatewaySession{
		Platform:      msg.Platform,
		UserID:        msg.UserID,
		ChatID:        msg.ChatID,
		Authenticated: !gs.config.Security.RequirePIN,
		LastActive:    time.Now(),
		DailyCount:    0,
		DailyDate:     time.Now().Format("2006-01-02"),
		Messages:      []GatewayMessage{},
	}
	gs.sessions[key] = session
	return session
}

// handleApprovalCommand 处理 /approve|/deny <id> 审批命令（绑定了 Bot 的会话）
// 返回 nil 表示不是审批命令（继续正常流程）
func (gs *GatewayService) handleApprovalCommand(msg IncomingMessage, session *GatewaySession) *OutgoingMessage {
	if gs.approvals == nil || session.BotID == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)
	approvalID := 0
	approve := false
	switch {
	case strings.HasPrefix(text, "/approve "), strings.HasPrefix(text, "/agree "):
		approve = true
		_, _ = fmt.Sscanf(text, "/approve %d", &approvalID)
		if approvalID == 0 {
			_, _ = fmt.Sscanf(text, "/agree %d", &approvalID)
		}
	case strings.HasPrefix(text, "/deny "), strings.HasPrefix(text, "/reject "):
		_, _ = fmt.Sscanf(text, "/deny %d", &approvalID)
		if approvalID == 0 {
			_, _ = fmt.Sscanf(text, "/reject %d", &approvalID)
		}
	default:
		return nil
	}
	if approvalID <= 0 {
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: "用法：/approve <编号> 或 /deny <编号>"}
	}

	reply, err := gs.approvals.DecideApproval(session.BotID, uint(approvalID), approve, msg.UserID)
	if err != nil {
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: "❌ " + err.Error()}
	}
	return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
		Text: reply, ReplyToMsgID: msg.MessageID}
}

func (gs *GatewayService) handleSpecialCommand(msg IncomingMessage, session *GatewaySession) *OutgoingMessage {
	text := msg.Text

	// PIN 验证
	if gs.config.Security.RequirePIN && !session.Authenticated {
		if text == gs.config.Security.PIN {
			session.Authenticated = true
			return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
				Text: "✅ 验证成功！现在可以开始对话了。"}
		}
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: "🔐 请输入 PIN 码以验证身份："}
	}

	switch text {
	case "/start", "/help":
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: "🤖 RDE AI 助手\n\n可用命令：\n/help - 显示帮助\n/status - 查看 NAS 状态\n/clear - 清除对话历史\n/disk - 查看磁盘状态\n/docker - 查看容器状态\n\n直接发送消息即可与 AI 对话。"}
	case "/clear":
		session.Messages = []GatewayMessage{}
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: "✅ 对话历史已清除"}
	case "/status":
		resultJSON := gs.skills.ExecuteToolCall("get_system_info", "{}")
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: fmt.Sprintf("📊 系统状态\n\n%s", formatToolResult(resultJSON))}
	case "/disk":
		resultJSON := gs.skills.ExecuteToolCall("get_disk_usage", "{}")
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: fmt.Sprintf("💾 磁盘状态\n\n%s", formatToolResult(resultJSON))}
	case "/docker":
		resultJSON := gs.skills.ExecuteToolCall("get_docker_status", "{}")
		return &OutgoingMessage{Platform: msg.Platform, ChatID: msg.ChatID, UserID: msg.UserID,
			Text: fmt.Sprintf("🐳 Docker 状态\n\n%s", formatToolResult(resultJSON))}
	}
	return nil
}

// resolveBotForSession 按消息渠道解析绑定的 Bot，并同步到会话
// 匹配规则：Bot.Channels 中 platform 相等且 chat_id 或 user_id 任一命中
func (gs *GatewayService) resolveBotForSession(msg IncomingMessage, session *GatewaySession) *BotInfo {
	if gs.botResolver == nil {
		return nil
	}
	bot := gs.botResolver.ResolveBotByChannel(string(msg.Platform), msg.ChatID, msg.UserID)
	if bot == nil {
		session.BotID = ""
		return nil
	}
	if session.BotID != bot.ID {
		gs.logger.Info("session bound to bot",
			zap.String("bot_id", bot.ID), zap.String("bot_name", bot.Name),
			zap.String("platform", string(msg.Platform)))
		// 绑定变化时切换身份，清空旧上下文避免角色串扰
		session.Messages = []GatewayMessage{}
	}
	session.BotID = bot.ID
	return bot
}

func (gs *GatewayService) callAI(ctx context.Context, session *GatewaySession, bot *BotInfo) (string, error) {
	// Bot 会话：职责/技能 system prompt 由 ChatWithToolsForBot 注入
	if bot != nil {
		response, err := gs.aiService.ChatWithToolsForBot(ctx, session.Messages, bot)
		if err != nil {
			return "", err
		}
		return response, nil
	}

	systemPrompt := `你是 RDE NAS 系统的 AI 助手，正在通过社交平台与用户交流。
保持回复简洁（适合移动端阅读），使用 emoji 增强可读性。
你可以使用各种工具来查询和管理 NAS 系统。`

	messages := make([]GatewayMessage, 0, len(session.Messages)+1)
	messages = append(messages, GatewayMessage{Role: "system", Content: systemPrompt})
	messages = append(messages, session.Messages...)

	response, err := gs.aiService.ChatWithTools(ctx, messages)
	if err != nil {
		return "", err
	}
	return response, nil
}

// GetAdapter 获取适配器
func (gs *GatewayService) GetAdapter(platform PlatformType) PlatformAdapter {
	return gs.adapters[platform]
}

// GetStatus 获取网关状态
func (gs *GatewayService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":   gs.config.Enabled,
		"platforms": map[string]interface{}{},
	}
	platforms := status["platforms"].(map[string]interface{})
	for platform, adapter := range gs.adapters {
		platforms[string(platform)] = map[string]interface{}{"enabled": adapter.IsEnabled()}
	}
	gs.sessionMu.RLock()
	status["active_sessions"] = len(gs.sessions)
	gs.sessionMu.RUnlock()
	return status
}

// 辅助函数

func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := cryptorand.Read(b); err != nil {
		// fallback should never happen
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func formatToolResult(jsonResult string) string {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonResult), &result); err != nil {
		return jsonResult
	}
	if errMsg, ok := result["error"].(string); ok {
		return "❌ " + errMsg
	}
	if data, ok := result["data"]; ok {
		formatted, _ := json.MarshalIndent(data, "", "  ")
		return string(formatted)
	}
	formatted, _ := json.MarshalIndent(result, "", "  ")
	return string(formatted)
}
