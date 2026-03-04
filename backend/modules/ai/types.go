// Package ai AI 类型定义
package ai

import "time"

// Provider AI 提供商类型
type Provider string

const (
	ProviderOllama     Provider = "ollama"
	ProviderOpenAI     Provider = "openai"
	ProviderClaude     Provider = "claude"
	ProviderGemini     Provider = "gemini"
	ProviderDeepSeek   Provider = "deepseek"
	ProviderZhipu      Provider = "zhipu"
	ProviderQwen       Provider = "qwen"
	ProviderMoonshot   Provider = "moonshot"
	ProviderGroq       Provider = "groq"
	ProviderOpenRouter Provider = "openrouter"
)

// 默认配置常量
const (
	DefaultProviderID   = "deepseek"
	DefaultProviderName = "DeepSeek"
	DefaultDeepSeekURL  = "https://api.deepseek.com"
	DefaultOllamaID     = "ollama-local"
	DefaultOllamaName   = "Local Ollama"
	DefaultOllamaURL    = "http://127.0.0.1:11434"
	DefaultModel        = "deepseek-chat"
	DefaultOllamaModel  = "qwen2.5:1.5b"
	DefaultSystemPrompt = "你是 RDE NAS 系统的 AI 助手，可以帮助用户管理和监控 NAS。"
	DefaultTemperature  = 0.7
	DefaultMaxTokens    = 2048
)

// ProviderConfig 提供商配置
type ProviderConfig struct {
	ID       string   `json:"id"`
	Provider Provider `json:"provider"`
	Name     string   `json:"name"`
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key,omitempty"`
	Models   []string `json:"models,omitempty"`
	Enabled  bool     `json:"enabled"`
}

// Model AI 模型
type Model struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     Provider `json:"provider"`
	ProviderID   string   `json:"provider_id"`
	Description  string   `json:"description,omitempty"`
	ContextLen   int      `json:"context_length,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// Conversation 对话
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	ModelID   string    `json:"model_id"`
	Provider  Provider  `json:"provider"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message 消息
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // system, user, assistant
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	ProviderID     string    `json:"provider_id"`
	Model          string    `json:"model" binding:"required"`
	Messages       []Message `json:"messages" binding:"required"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Stream         bool      `json:"stream"`
	MaxTokens      int       `json:"max_tokens,omitempty"`
	Temperature    float64   `json:"temperature,omitempty"`
	TopP           float64   `json:"top_p,omitempty"`
	SystemPrompt   string    `json:"system_prompt,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	ID             string  `json:"id"`
	ConversationID string  `json:"conversation_id"`
	Model          string  `json:"model"`
	Content        string  `json:"content"`
	FinishReason   string  `json:"finish_reason,omitempty"`
	Usage          *Usage  `json:"usage,omitempty"`
}

// Usage 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk 流式响应块
type StreamChunk struct {
	ID           string `json:"id"`
	Delta        string `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
	Error        string `json:"error,omitempty"`
}

// CreateProviderRequest 创建提供商请求
type CreateProviderRequest struct {
	Provider Provider `json:"provider" binding:"required"`
	Name     string   `json:"name" binding:"required"`
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
}

// UpdateProviderRequest 更新提供商请求
type UpdateProviderRequest struct {
	Name    string `json:"name,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// CreateConversationRequest 创建对话请求
type CreateConversationRequest struct {
	Title    string   `json:"title"`
	ModelID  string   `json:"model_id"`
	Provider Provider `json:"provider"`
}

// UpdateConversationRequest 更新对话请求
type UpdateConversationRequest struct {
	Title string `json:"title"`
}

// OllamaModel Ollama 模型
type OllamaModel struct {
	Name       string            `json:"name"`
	ModifiedAt time.Time         `json:"modified_at"`
	Size       int64             `json:"size"`
	Digest     string            `json:"digest"`
	Details    OllamaModelDetail `json:"details"`
}

// OllamaModelDetail Ollama 模型详情
type OllamaModelDetail struct {
	Format           string   `json:"format"`
	Family           string   `json:"family"`
	Families         []string `json:"families"`
	ParameterSize    string   `json:"parameter_size"`
	QuantizationLevel string  `json:"quantization_level"`
}

// OllamaChatRequest Ollama 聊天请求
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *OllamaOptions  `json:"options,omitempty"`
}

// OllamaMessage Ollama 消息
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions Ollama 选项
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// OllamaChatResponse Ollama 聊天响应
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt time.Time     `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
	TotalDur  int64         `json:"total_duration,omitempty"`
	EvalCount int           `json:"eval_count,omitempty"`
}

// OpenAIChatRequest OpenAI 聊天请求
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
}

// OpenAIMessage OpenAI 消息
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatResponse OpenAI 聊天响应
type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIChoice OpenAI 选择
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage OpenAI 使用量
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OllamaPullRequest Ollama 拉取模型请求
type OllamaPullRequest struct {
	Model string `json:"model" binding:"required"`
}

// OllamaDeleteRequest Ollama 删除模型请求
type OllamaDeleteRequest struct {
	Model string `json:"model" binding:"required"`
}

// ==================== Function Calling ====================

// ToolDefinition OpenAI 风格的工具定义
type ToolDefinition struct {
	Type     string      `json:"type"` // "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall LLM 返回的工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ChatMessageWithTools 支持工具调用的消息
type ChatMessageWithTools struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// OpenAIChatRequestWithTools 带工具的 OpenAI 聊天请求
type OpenAIChatRequestWithTools struct {
	Model       string                 `json:"model"`
	Messages    []ChatMessageWithTools `json:"messages"`
	Tools       []ToolDefinition       `json:"tools,omitempty"`
	Stream      bool                   `json:"stream"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
}

// OpenAIChatResponseWithTools 带工具调用的响应
type OpenAIChatResponseWithTools struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      ChatMessageWithTools `json:"message"`
		Delta        ChatMessageWithTools `json:"delta"`
		FinishReason string               `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// ==================== Config / Status ====================

// AIConfig AI 全局配置
type AIConfig struct {
	Enabled         bool    `json:"enabled"`
	DefaultProvider string  `json:"default_provider"`
	DefaultModel    string  `json:"default_model"`
	SystemPrompt    string  `json:"system_prompt"`
	MaxTokens       int     `json:"max_tokens"`
	Temperature     float64 `json:"temperature"`
	EnableTools     bool    `json:"enable_tools"`
}

// AIConfigUpdateRequest 更新配置请求
type AIConfigUpdateRequest struct {
	Enabled         *bool    `json:"enabled,omitempty"`
	DefaultProvider string   `json:"default_provider,omitempty"`
	DefaultModel    string   `json:"default_model,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	EnableTools     *bool    `json:"enable_tools,omitempty"`
}

// ProviderStatus Provider 状态
type ProviderStatus struct {
	ID       string   `json:"id"`
	Provider Provider `json:"provider"`
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	Status   string   `json:"status"` // connected, disconnected, unknown
	Error    string   `json:"error,omitempty"`
}

// AIServiceStatus 服务状态
type AIServiceStatus struct {
	Enabled           bool             `json:"enabled"`
	Providers         []ProviderStatus `json:"providers"`
	ConversationCount int              `json:"conversation_count"`
	DefaultProvider   string           `json:"default_provider"`
	DefaultModel      string           `json:"default_model"`
	ToolsEnabled      bool             `json:"tools_enabled"`
}

// ==================== Setup ====================

// SetupStatus 向导状态
type SetupStatus string

const (
	SetupPending    SetupStatus = "pending"
	SetupInProgress SetupStatus = "in_progress"
	SetupComplete   SetupStatus = "complete"
)

// SetupStep 向导步骤
type SetupStep string

const (
	StepEnvironment SetupStep = "environment"
	StepModel       SetupStep = "model"
	StepSkills      SetupStep = "skills"
	StepComplete    SetupStep = "complete"
)

// SetupState Setup 状态
type SetupState struct {
	Status        SetupStatus `json:"status"`
	CurrentStep   SetupStep   `json:"current_step"`
	SelectedModel string      `json:"selected_model"`
	SkillsEnabled []string    `json:"skills_enabled"`
	StartedAt     time.Time   `json:"started_at"`
	CompletedAt   *time.Time  `json:"completed_at,omitempty"`
}

// EnvironmentCheck 环境检查结果
type EnvironmentCheck struct {
	Docker    ComponentStatus `json:"docker"`
	DiskSpace DiskSpaceStatus `json:"disk_space"`
	Network   NetworkStatus   `json:"network"`
	GPU       ComponentStatus `json:"gpu"`
	OS        string          `json:"os"`
	Arch      string          `json:"arch"`
}

// ComponentStatus 组件状态
type ComponentStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ready, not_installed, not_available
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

// DockerServiceCheck Docker 服务检查
type DockerServiceCheck struct {
	Running bool `json:"running"`
}

// DiskSpaceStatus 磁盘空间状态
type DiskSpaceStatus struct {
	Available  string `json:"available"`
	Total      string `json:"total,omitempty"`
	Required   string `json:"required"`
	Sufficient bool   `json:"sufficient"`
}

// NetworkStatus 网络状态
type NetworkStatus struct {
	Internet        bool `json:"internet"`
	DockerHub       bool `json:"docker_hub"`
	OllamaReachable bool `json:"ollama_reachable"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        string `json:"size"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
	MinRAM      string `json:"min_ram,omitempty"`
}

// DownloadProgress 下载进度
type DownloadProgress struct {
	Model      string  `json:"model"`
	Status     string  `json:"status"`
	Percentage float64 `json:"percentage"`
	Log        string  `json:"log,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// DownloadModelRequest 下载模型请求
type DownloadModelRequest struct {
	Model string `json:"model" binding:"required"`
}

// SetupCompleteRequest 完成设置请求
type SetupCompleteRequest struct {
	SelectedModel string   `json:"selected_model"`
	SkillsEnabled []string `json:"skills_enabled"`
}

// ==================== Skills ====================

// Skill 技能定义
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Category    string `json:"category,omitempty"`
}

// SkillsConfig 技能配置请求
type SkillsConfig struct {
	EnabledSkills []string `json:"enabledSkills"`
}

// SkillRequest 技能执行请求
type SkillRequest struct {
	SkillID   string                 `json:"skill_id" binding:"required"`
	Action    string                 `json:"action"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// SkillResponse 技能执行响应
type SkillResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Summary string      `json:"summary,omitempty"`
}

// ==================== Gateway ====================

// PlatformType 平台类型
type PlatformType string

const (
	PlatformWecom    PlatformType = "wecom"
	PlatformTelegram PlatformType = "telegram"
	PlatformWebhook  PlatformType = "webhook"
)

// GatewayConfig 消息网关配置
type GatewayConfig struct {
	Enabled  bool           `json:"enabled"`
	Wecom    WecomConfig    `json:"wecom"`
	Telegram TelegramConfig `json:"telegram"`
	Webhook  WebhookConfig  `json:"webhook"`
	Security SecurityConfig `json:"security"`
}

// WecomConfig 企业微信配置
type WecomConfig struct {
	Enabled     bool   `json:"enabled"`
	CorpID      string `json:"corp_id"`
	AgentID     int    `json:"agent_id"`
	Secret      string `json:"secret"`
	Token       string `json:"token"`
	EncodingKey string `json:"encoding_key"`
	CallbackURL string `json:"callback_url"`
}

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	Enabled    bool   `json:"enabled"`
	BotToken   string `json:"bot_token"`
	WebhookURL string `json:"webhook_url"`
	UseWebhook bool   `json:"use_webhook"`
	ProxyMode  string `json:"proxy_mode"` // off, system, custom
	ProxyURL   string `json:"proxy_url"`
}

// WebhookConfig 通用 Webhook 配置
type WebhookConfig struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	AllowedUsers        []string `json:"allowed_users"`
	RequireConfirmation []string `json:"require_confirmation"`
	DailyLimit          int      `json:"daily_limit"`
	RequirePIN          bool     `json:"require_pin"`
	PIN                 string   `json:"pin"`
}

// IncomingMessage 传入消息
type IncomingMessage struct {
	Platform       PlatformType `json:"platform"`
	UserID         string       `json:"user_id"`
	UserName       string       `json:"user_name"`
	ChatID         string       `json:"chat_id"`
	Text           string       `json:"text"`
	MessageID      string       `json:"message_id"`
	Timestamp      time.Time    `json:"timestamp"`
	ReplyToMsgID   string       `json:"reply_to_msg_id,omitempty"`
	IsGroupMessage bool         `json:"is_group_message"`
}

// OutgoingMessage 发出消息
type OutgoingMessage struct {
	Platform     PlatformType `json:"platform"`
	ChatID       string       `json:"chat_id"`
	UserID       string       `json:"user_id"`
	Text         string       `json:"text"`
	ReplyToMsgID string       `json:"reply_to_msg_id,omitempty"`
	MediaURL     string       `json:"media_url,omitempty"`
	MediaType    string       `json:"media_type,omitempty"`
}

// GatewaySession 网关会话
type GatewaySession struct {
	Platform      PlatformType     `json:"platform"`
	UserID        string           `json:"user_id"`
	ChatID        string           `json:"chat_id"`
	Authenticated bool             `json:"authenticated"`
	LastActive    time.Time        `json:"last_active"`
	DailyCount    int              `json:"daily_count"`
	DailyDate     string           `json:"daily_date"`
	Messages      []GatewayMessage `json:"messages"`
}

// GatewayMessage 网关消息
type GatewayMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ==================== Alerts ====================

// AlertType 告警类型
type AlertType string

const (
	AlertDiskFull      AlertType = "disk_full"
	AlertDiskWarning   AlertType = "disk_warning"
	AlertContainerDown AlertType = "container_down"
	AlertHighCPU       AlertType = "high_cpu"
	AlertHighMemory    AlertType = "high_memory"
	AlertHighTemp      AlertType = "high_temp"
	AlertSmartWarning  AlertType = "smart_warning"
	AlertRaidDegraded  AlertType = "raid_degraded"
	AlertServiceDown   AlertType = "service_down"
)

// AlertLevel 告警等级
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled          bool           `json:"enabled"`
	CheckInterval    int            `json:"check_interval"`
	DiskWarningPct   float64        `json:"disk_warning_pct"`
	DiskCriticalPct  float64        `json:"disk_critical_pct"`
	CPUWarningPct    float64        `json:"cpu_warning_pct"`
	MemoryWarningPct float64        `json:"memory_warning_pct"`
	TempWarningC     float64        `json:"temp_warning_c"`
	EnabledAlerts    []AlertType    `json:"enabled_alerts"`
	NotifyPlatforms  []PlatformType `json:"notify_platforms"`
	NotifyUsers      []string       `json:"notify_users"`
	QuietHoursStart  int            `json:"quiet_hours_start"`
	QuietHoursEnd    int            `json:"quiet_hours_end"`
	CooldownMinutes  int            `json:"cooldown_minutes"`
}

// Alert 告警
type Alert struct {
	ID         string     `json:"id"`
	Type       AlertType  `json:"type"`
	Level      AlertLevel `json:"level"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	Source     string     `json:"source"`
	Value      float64    `json:"value"`
	Threshold  float64    `json:"threshold"`
	Timestamp  time.Time  `json:"timestamp"`
	Notified   bool       `json:"notified"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// ==================== Voice ====================

// VoiceConfig 语音配置
type VoiceConfig struct {
	STTProvider string `json:"stt_provider"` // whisper, azure
	TTSProvider string `json:"tts_provider"` // edge, openai, azure
	STTModel    string `json:"stt_model"`
	TTSModel    string `json:"tts_model"`
	TTSVoice    string `json:"tts_voice"`
	Language    string `json:"language,omitempty"` // zh-CN, en-US, etc.
	OpenAIKey   string `json:"openai_key,omitempty"`
	AzureKey    string `json:"azure_key,omitempty"`
	AzureRegion string `json:"azure_region,omitempty"`
}

// SaveMessagesRequest 保存消息请求（流式聊天后保存）
type SaveMessagesRequest struct {
	UserContent      string `json:"userContent"`
	AssistantContent string `json:"assistantContent"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
}
