// Package ai AI 服务
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service AI 服务
type Service struct {
	logger        *zap.Logger
	dataDir       string
	providers     map[string]*ProviderConfig
	conversations map[string]*Conversation
	mu            sync.RWMutex
	client        *http.Client
	config        *AIConfig
	skills        *SkillsService
	saveTimer     *time.Timer
	saveDirty     bool
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, dataDir string) *Service {
	os.MkdirAll(dataDir, 0755)

	s := &Service{
		logger:        logger,
		dataDir:       dataDir,
		providers:     make(map[string]*ProviderConfig),
		conversations: make(map[string]*Conversation),
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}

	s.loadConfig()
	s.loadProviders()
	s.loadConversations()

	// 添加默认提供商：在线优先，本地兜底
	if len(s.providers) == 0 {
		s.providers[DefaultProviderID] = &ProviderConfig{
			ID:       DefaultProviderID,
			Provider: ProviderDeepSeek,
			Name:     DefaultProviderName,
			BaseURL:  DefaultDeepSeekURL,
			Enabled:  true,
		}
		s.providers[DefaultOllamaID] = &ProviderConfig{
			ID:       DefaultOllamaID,
			Provider: ProviderOllama,
			Name:     DefaultOllamaName,
			BaseURL:  DefaultOllamaURL,
			Enabled:  true,
		}
		s.saveProviders()
	}

	return s
}

// GetProviders 获取提供商列表
// providerSortOrder 在线提供商排序靠前，本地排后
var providerSortOrder = map[Provider]int{
	ProviderDeepSeek:   0,
	ProviderQwen:       1,
	ProviderOpenAI:     2,
	ProviderClaude:     3,
	ProviderGemini:     4,
	ProviderZhipu:      5,
	ProviderMoonshot:   6,
	ProviderGroq:       7,
	ProviderOpenRouter: 8,
	ProviderOllama:     90,
}

func (s *Service) GetProviders() []*ProviderConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := make([]*ProviderConfig, 0, len(s.providers))
	for _, p := range s.providers {
		// 隐藏 API Key
		pc := *p
		if pc.APIKey != "" {
			pc.APIKey = "***"
		}
		providers = append(providers, &pc)
	}
	// 按 providerSortOrder 排序：在线优先，本地靠后
	sort.Slice(providers, func(i, j int) bool {
		oi, oj := providerSortOrder[providers[i].Provider], providerSortOrder[providers[j].Provider]
		if oi != oj {
			return oi < oj
		}
		return providers[i].Name < providers[j].Name
	})
	return providers
}

// GetProvider 获取提供商
func (s *Service) GetProvider(id string) (*ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return p, nil
}

// CreateProvider 创建提供商
func (s *Service) CreateProvider(req CreateProviderRequest) (*ProviderConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()[:8]
	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = s.getDefaultBaseURL(req.Provider)
	}

	provider := &ProviderConfig{
		ID:       id,
		Provider: req.Provider,
		Name:     req.Name,
		BaseURL:  baseURL,
		APIKey:   req.APIKey,
		Enabled:  true,
	}

	s.providers[id] = provider
	s.saveProviders()

	return provider, nil
}

// UpdateProvider 更新提供商
func (s *Service) UpdateProvider(id string, req UpdateProviderRequest) (*ProviderConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.BaseURL != "" {
		p.BaseURL = req.BaseURL
	}
	if req.APIKey != "" {
		p.APIKey = req.APIKey
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}

	s.saveProviders()
	return p, nil
}

// DeleteProvider 删除提供商
func (s *Service) DeleteProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.providers[id]; !ok {
		return fmt.Errorf("provider not found: %s", id)
	}

	delete(s.providers, id)
	s.saveProviders()
	return nil
}

// GetModels 获取模型列表
func (s *Service) GetModels(providerID string) ([]Model, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	switch provider.Provider {
	case ProviderOllama:
		return s.getOllamaModels(provider)
	default:
		return s.getOpenAIModels(provider)
	}
}

// Chat 聊天
func (s *Service) Chat(req ChatRequest) (*ChatResponse, error) {
	providerID := req.ProviderID
	if providerID == "" {
		providerID = DefaultProviderID
	}

	s.mu.RLock()
	provider, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	switch provider.Provider {
	case ProviderOllama:
		return s.chatOllama(provider, req)
	default:
		return s.chatOpenAI(provider, req)
	}
}

// ChatStream 流式聊天
func (s *Service) ChatStream(req ChatRequest, stream chan<- StreamChunk) error {
	defer close(stream)

	providerID := req.ProviderID
	if providerID == "" {
		providerID = DefaultProviderID
	}

	s.mu.RLock()
	provider, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	// 对非 Ollama 的 OpenAI 兼容提供商，启用工具时使用 streamWithToolCalls
	if provider.Provider != ProviderOllama && s.config.EnableTools && s.skills != nil {
		return s.streamWithToolCalls(provider, req.Model, req, stream)
	}

	switch provider.Provider {
	case ProviderOllama:
		return s.streamOllamaInternal(provider, req, stream)
	default:
		return s.streamOpenAIInternal(provider, req, stream)
	}
}

// GetConversations 获取对话列表
func (s *Service) GetConversations() []*Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	convs := make([]*Conversation, 0, len(s.conversations))
	for _, c := range s.conversations {
		// 不返回消息内容
		conv := *c
		conv.Messages = nil
		convs = append(convs, &conv)
	}
	return convs
}

// GetConversation 获取对话（返回深拷贝，避免并发问题）
func (s *Service) GetConversation(id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.conversations[id]
	if !ok {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}
	// 深拷贝，避免 JSON 序列化时与 AddMessage 产生竞态
	conv := *c
	conv.Messages = make([]Message, len(c.Messages))
	copy(conv.Messages, c.Messages)
	return &conv, nil
}

// CreateConversation 创建对话
func (s *Service) CreateConversation(req CreateConversationRequest) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	conv := &Conversation{
		ID:        id,
		Title:     req.Title,
		ModelID:   req.ModelID,
		Provider:  req.Provider,
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if conv.Title == "" {
		conv.Title = "New Conversation"
	}

	s.conversations[id] = conv
	s.saveConversations()

	return conv, nil
}

// UpdateConversation 更新对话
func (s *Service) UpdateConversation(id string, req UpdateConversationRequest) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.conversations[id]
	if !ok {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}

	if req.Title != "" {
		c.Title = req.Title
	}
	c.UpdatedAt = time.Now()

	s.saveConversations()
	return c, nil
}

// DeleteConversation 删除对话
func (s *Service) DeleteConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conversations[id]; !ok {
		return fmt.Errorf("conversation not found: %s", id)
	}

	delete(s.conversations, id)
	s.saveConversations()
	return nil
}

// AddMessage 添加消息到对话
func (s *Service) AddMessage(convID string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.conversations[convID]
	if !ok {
		return fmt.Errorf("conversation not found: %s", convID)
	}

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()

	// 消息保存使用同步写入，确保数据不会因进程异常退出而丢失
	s.saveConversationsSync()
	return nil
}

// PullOllamaModel 拉取 Ollama 模型
func (s *Service) PullOllamaModel(providerID, model string) error {
	s.mu.RLock()
	provider, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	if provider.Provider != ProviderOllama {
		return fmt.Errorf("provider is not ollama")
	}

	url := provider.BaseURL + "/api/pull"
	body := map[string]string{"name": model}
	data, _ := json.Marshal(body)

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 等待拉取完成
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// 只是消费流
	}

	return nil
}

// DeleteOllamaModel 删除 Ollama 模型
func (s *Service) DeleteOllamaModel(providerID, model string) error {
	s.mu.RLock()
	provider, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	if provider.Provider != ProviderOllama {
		return fmt.Errorf("provider is not ollama")
	}

	url := provider.BaseURL + "/api/delete"
	body := map[string]string{"name": model}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("DELETE", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete model failed: %d", resp.StatusCode)
	}

	return nil
}

func (s *Service) getOllamaModels(provider *ProviderConfig) ([]Model, error) {
	url := provider.BaseURL + "/api/tags"
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []OllamaModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]Model, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, Model{
			ID:         m.Name,
			Name:       m.Name,
			Provider:   ProviderOllama,
			ProviderID: provider.ID,
		})
	}
	return models, nil
}

// 预定义的在线服务模型列表（无需 API Key 即可知道支持的模型）
var predefinedModels = map[Provider][]string{
	ProviderDeepSeek: {"deepseek-chat", "deepseek-reasoner"},
	ProviderOpenAI:   {"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
	ProviderClaude:   {"claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"},
	ProviderQwen:     {"qwen-turbo", "qwen-plus", "qwen-max"},
	ProviderGemini:   {"gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"},
	ProviderZhipu:    {"glm-4-flash", "glm-4-plus", "glm-4"},
}

func (s *Service) getOpenAIModels(provider *ProviderConfig) ([]Model, error) {
	// 无 API Key 时返回预定义模型列表
	if provider.APIKey == "" {
		if predef, ok := predefinedModels[provider.Provider]; ok {
			models := make([]Model, 0, len(predef))
			for _, id := range predef {
				models = append(models, Model{
					ID:         id,
					Name:       id,
					Provider:   provider.Provider,
					ProviderID: provider.ID,
				})
			}
			return models, nil
		}
		return nil, fmt.Errorf("API key required for %s", provider.Provider)
	}

	url := provider.BaseURL + "/v1/models"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]Model, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, Model{
			ID:         m.ID,
			Name:       m.ID,
			Provider:   provider.Provider,
			ProviderID: provider.ID,
		})
	}
	return models, nil
}

func (s *Service) chatOllama(provider *ProviderConfig, req ChatRequest) (*ChatResponse, error) {
	ollamaReq := OllamaChatRequest{
		Model:  req.Model,
		Stream: false,
	}

	if req.SystemPrompt != "" {
		ollamaReq.Messages = append(ollamaReq.Messages, OllamaMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, m := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, OllamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	if req.MaxTokens > 0 || req.Temperature > 0 || req.TopP > 0 {
		ollamaReq.Options = &OllamaOptions{
			NumPredict:  req.MaxTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
		}
	}

	data, _ := json.Marshal(ollamaReq)
	url := provider.BaseURL + "/api/chat"

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &ChatResponse{
		ID:             uuid.New().String(),
		ConversationID: req.ConversationID,
		Model:          req.Model,
		Content:        result.Message.Content,
	}, nil
}

func (s *Service) chatOpenAI(provider *ProviderConfig, req ChatRequest) (*ChatResponse, error) {
	openaiReq := OpenAIChatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      false,
	}

	if req.SystemPrompt != "" {
		openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, m := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	data, _ := json.Marshal(openaiReq)
	url := provider.BaseURL + "/v1/chat/completions"

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	chatResp := &ChatResponse{
		ID:             result.ID,
		ConversationID: req.ConversationID,
		Model:          req.Model,
		Content:        content,
	}

	if result.Usage != nil {
		chatResp.Usage = &Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		}
	}

	return chatResp, nil
}

func (s *Service) streamOllamaInternal(provider *ProviderConfig, req ChatRequest, stream chan<- StreamChunk) error {
	ollamaReq := OllamaChatRequest{
		Model:  req.Model,
		Stream: true,
	}

	if req.SystemPrompt != "" {
		ollamaReq.Messages = append(ollamaReq.Messages, OllamaMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, m := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, OllamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	data, _ := json.Marshal(ollamaReq)
	url := provider.BaseURL + "/api/chat"

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk OllamaChatResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}

		sc := StreamChunk{
			ID:    uuid.New().String(),
			Delta: chunk.Message.Content,
		}
		if chunk.Done {
			sc.FinishReason = "stop"
		}
		stream <- sc
	}

	return nil
}

func (s *Service) streamOpenAI(provider *ProviderConfig, req ChatRequest, stream chan<- StreamChunk) error {
	return s.streamOpenAIInternal(provider, req, stream)
}

func (s *Service) streamOpenAIInternal(provider *ProviderConfig, req ChatRequest, stream chan<- StreamChunk) error {
	openaiReq := OpenAIChatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      true,
	}

	if req.SystemPrompt != "" {
		openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, m := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	data, _ := json.Marshal(openaiReq)
	url := provider.BaseURL + "/v1/chat/completions"

	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk OpenAIChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			sc := StreamChunk{
				ID:           chunk.ID,
				FinishReason: chunk.Choices[0].FinishReason,
			}
			if chunk.Choices[0].Delta != nil {
				sc.Delta = chunk.Choices[0].Delta.Content
			}
			stream <- sc
		}
	}

	return nil
}

func (s *Service) getDefaultBaseURL(provider Provider) string {
	switch provider {
	case ProviderOllama:
		return DefaultOllamaURL
	case ProviderOpenAI:
		return "https://api.openai.com"
	case ProviderClaude:
		return "https://api.anthropic.com"
	case ProviderGemini:
		return "https://generativelanguage.googleapis.com"
	case ProviderDeepSeek:
		return DefaultDeepSeekURL
	case ProviderZhipu:
		return "https://open.bigmodel.cn/api/paas"
	case ProviderQwen:
		return "https://dashscope.aliyuncs.com/compatible-mode"
	case ProviderMoonshot:
		return "https://api.moonshot.cn"
	case ProviderGroq:
		return "https://api.groq.com/openai"
	case ProviderOpenRouter:
		return "https://openrouter.ai/api"
	default:
		return ""
	}
}

// SetSkills 设置技能服务引用
func (s *Service) SetSkills(skills *SkillsService) { s.skills = skills }

// GetAIConfig 获取 AI 配置
func (s *Service) GetAIConfig() *AIConfig { return s.config }

// UpdateAIConfig 更新 AI 配置
func (s *Service) UpdateAIConfig(req AIConfigUpdateRequest) error {
	if req.DefaultProvider != "" {
		s.config.DefaultProvider = req.DefaultProvider
	}
	if req.DefaultModel != "" {
		s.config.DefaultModel = req.DefaultModel
	}
	if req.SystemPrompt != "" {
		s.config.SystemPrompt = req.SystemPrompt
	}
	if req.Temperature != nil {
		s.config.Temperature = *req.Temperature
	}
	if req.MaxTokens != nil {
		s.config.MaxTokens = *req.MaxTokens
	}
	if req.EnableTools != nil {
		s.config.EnableTools = *req.EnableTools
	}
	return s.saveConfig()
}

// GetAIStatus 获取 AI 服务状态
func (s *Service) GetAIStatus() *AIServiceStatus {
	status := &AIServiceStatus{
		Providers: make([]ProviderStatus, 0),
	}

	// 复制 provider 列表后释放锁，避免 HTTP 探测时长时间持锁
	s.mu.RLock()
	providersCopy := make([]*ProviderConfig, 0, len(s.providers))
	for _, p := range s.providers {
		cp := *p
		providersCopy = append(providersCopy, &cp)
	}
	status.ConversationCount = len(s.conversations)
	s.mu.RUnlock()

	// 并行检查所有 provider 的状态，使用短超时
	type result struct {
		idx int
		ps  ProviderStatus
	}
	results := make(chan result, len(providersCopy))

	for i, p := range providersCopy {
		go func(idx int, p *ProviderConfig) {
			ps := ProviderStatus{ID: p.ID, Name: p.Name, Provider: p.Provider, Enabled: p.Enabled, Status: "unknown"}
			if p.Enabled {
				// 只对 Ollama 类型检查连接状态
				if p.Provider == ProviderOllama {
					if err := s.checkProviderStatusWithTimeout(p, 3*time.Second); err == nil {
						ps.Status = "connected"
					} else {
						ps.Status = "disconnected"
						ps.Error = err.Error()
					}
				} else {
					// 在线服务只需要有 API Key 即认为已配置
					if p.APIKey != "" {
						ps.Status = "configured"
					} else {
						ps.Status = "not_configured"
					}
				}
			}
			results <- result{idx: idx, ps: ps}
		}(i, p)
	}

	// 收集结果
	providerStatuses := make([]ProviderStatus, len(providersCopy))
	for range providersCopy {
		r := <-results
		providerStatuses[r.idx] = r.ps
	}
	status.Providers = providerStatuses

	status.DefaultProvider = s.config.DefaultProvider
	status.DefaultModel = s.config.DefaultModel
	status.ToolsEnabled = s.config.EnableTools
	return status
}

// checkProviderStatusWithTimeout 带超时检查 provider 状态
func (s *Service) checkProviderStatusWithTimeout(p *ProviderConfig, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(p.BaseURL + "/api/tags")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ChatWithTools 带工具调用的聊天（用于网关）
func (s *Service) ChatWithTools(ctx context.Context, messages []GatewayMessage) (string, error) {
	providerID := s.config.DefaultProvider
	if providerID == "" {
		providerID = DefaultProviderID
	}

	s.mu.RLock()
	provider, ok := s.providers[providerID]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("provider not found: %s", providerID)
	}

	model := s.config.DefaultModel
	if model == "" {
		model = DefaultModel
	}

	// 转换消息格式
	toolMessages := make([]ChatMessageWithTools, 0, len(messages))
	for _, m := range messages {
		toolMessages = append(toolMessages, ChatMessageWithTools{Role: m.Role, Content: m.Content})
	}

	// 如果启用了工具，使用函数调用循环
	if s.config.EnableTools && s.skills != nil {
		return s.chatWithToolsLoop(provider, model, toolMessages)
	}

	// 普通聊天
	req := ChatRequest{
		ProviderID: providerID,
		Model:      model,
		Messages:   make([]Message, 0, len(messages)),
	}
	for _, m := range messages {
		req.Messages = append(req.Messages, Message{Role: m.Role, Content: m.Content})
	}

	resp, err := s.Chat(req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (s *Service) chatWithToolsLoop(provider *ProviderConfig, model string, messages []ChatMessageWithTools) (string, error) {
	tools := s.skills.GetToolDefinitions()

	for i := 0; i < 5; i++ { // 最多 5 轮工具调用
		reqBody := OpenAIChatRequestWithTools{
			Model:    model,
			Messages: messages,
			Tools:    tools,
			Stream:   false,
		}

		data, _ := json.Marshal(reqBody)
		url := provider.BaseURL + "/v1/chat/completions"

		httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		httpReq.Header.Set("Content-Type", "application/json")
		if provider.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
		}

		resp, err := s.client.Do(httpReq)
		if err != nil {
			return "", err
		}

		var result OpenAIChatResponseWithTools
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if len(result.Choices) == 0 {
			return "", fmt.Errorf("no response from model")
		}

		choice := result.Choices[0]

		// 没有工具调用，直接返回
		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		// 将 assistant 的消息（含 tool_calls）加入历史
		messages = append(messages, ChatMessageWithTools{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		})

		// 执行每个工具调用
		for _, tc := range choice.Message.ToolCalls {
			resultJSON := s.skills.ExecuteToolCall(tc.Function.Name, tc.Function.Arguments)
			messages = append(messages, ChatMessageWithTools{
				Role:       "tool",
				Content:    resultJSON,
				ToolCallID: tc.ID,
			})
		}
	}

	return "工具调用超过最大轮数", nil
}

// SaveMessages 保存消息到对话
func (s *Service) SaveMessages(convID string, req SaveMessagesRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.conversations[convID]
	if !ok {
		return fmt.Errorf("conversation not found: %s", convID)
	}

	if req.UserContent != "" {
		c.Messages = append(c.Messages, Message{
			ID:        uuid.New().String(),
			Role:      "user",
			Content:   req.UserContent,
			Timestamp: time.Now(),
		})
	}
	if req.AssistantContent != "" {
		c.Messages = append(c.Messages, Message{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   req.AssistantContent,
			Timestamp: time.Now(),
		})
	}
	c.UpdatedAt = time.Now()
	s.saveConversationsSync()
	return nil
}

func (s *Service) loadConfig() {
	s.config = &AIConfig{
		DefaultProvider: DefaultProviderID,
		DefaultModel:    DefaultModel,
		SystemPrompt:    DefaultSystemPrompt,
		Temperature:     DefaultTemperature,
		MaxTokens:       DefaultMaxTokens,
		EnableTools:     true,
	}
	file := filepath.Join(s.dataDir, "ai_config.json")
	data, err := os.ReadFile(file)
	if err == nil {
		json.Unmarshal(data, s.config)
	}
}

func (s *Service) saveConfig() error {
	file := filepath.Join(s.dataDir, "ai_config.json")
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := file + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, file)
}

func (s *Service) loadProviders() {
	file := filepath.Join(s.dataDir, "providers.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}

	var providers map[string]*ProviderConfig
	if json.Unmarshal(data, &providers) == nil {
		s.providers = providers
	}
}

func (s *Service) saveProviders() {
	file := filepath.Join(s.dataDir, "providers.json")
	data, err := json.MarshalIndent(s.providers, "", "  ")
	if err != nil {
		s.logger.Error("failed to marshal providers", zap.Error(err))
		return
	}
	tmpFile := file + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		s.logger.Error("failed to write providers", zap.Error(err))
		return
	}
	os.Rename(tmpFile, file)
}

func (s *Service) loadConversations() {
	file := filepath.Join(s.dataDir, "conversations.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}

	var convs map[string]*Conversation
	if json.Unmarshal(data, &convs) == nil {
		s.conversations = convs
	}
}

func (s *Service) saveConversations() {
	// 延迟写入：2 秒内多次调用合并为一次磁盘写入
	s.saveDirty = true
	if s.saveTimer == nil {
		s.saveTimer = time.AfterFunc(2*time.Second, s.flushConversations)
	} else {
		s.saveTimer.Reset(2 * time.Second)
	}
}

// saveConversationsSync 立即同步写入对话数据到磁盘（用于关键操作后确保数据不丢失）
func (s *Service) saveConversationsSync() {
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.saveDirty = true
	// 释放锁后立即写入
	go s.flushConversations()
}

// flushConversations 实际执行对话持久化写入
func (s *Service) flushConversations() {
	s.mu.Lock()
	if !s.saveDirty {
		s.mu.Unlock()
		return
	}
	s.saveDirty = false
	data, err := json.MarshalIndent(s.conversations, "", "  ")
	s.mu.Unlock()

	if err != nil {
		s.logger.Error("failed to marshal conversations", zap.Error(err))
		return
	}

	file := filepath.Join(s.dataDir, "conversations.json")
	tmpFile := file + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		s.logger.Error("failed to write conversations", zap.Error(err))
		return
	}
	os.Rename(tmpFile, file)
}

// FlushSave 立即将未保存的数据写入磁盘（用于关停时调用）
func (s *Service) FlushSave() {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.mu.Unlock()
	s.flushConversations()
}
