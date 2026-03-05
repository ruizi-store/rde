// Package ai AI 模块测试
package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestService(t *testing.T) *Service {
	t.Helper()

	tempDir := t.TempDir()
	logger := zap.NewNop()

	return NewService(logger, tempDir)
}

// ----- Provider 测试 -----

func TestService_GetProviders_DefaultOllama(t *testing.T) {
	svc := setupTestService(t)

	providers := svc.GetProviders()
	require.NotEmpty(t, providers)

	// 默认应有 deepseek 和 ollama-local
	var foundDeepSeek, foundOllama bool
	for _, p := range providers {
		if p.ID == "deepseek" {
			foundDeepSeek = true
			assert.Equal(t, ProviderDeepSeek, p.Provider)
			assert.Equal(t, "DeepSeek", p.Name)
		}
		if p.ID == "ollama-local" {
			foundOllama = true
			assert.Equal(t, ProviderOllama, p.Provider)
			assert.Equal(t, "Local Ollama", p.Name)
		}
	}
	assert.True(t, foundDeepSeek, "default deepseek provider should exist")
	assert.True(t, foundOllama, "default ollama provider should exist")

	// 在线提供商应该排在前面
	assert.Equal(t, "deepseek", providers[0].ID, "online provider should be first")
}

func TestService_CreateProvider(t *testing.T) {
	svc := setupTestService(t)

	req := CreateProviderRequest{
		Provider: ProviderOpenAI,
		Name:     "My OpenAI",
		APIKey:   "sk-test-key",
	}

	provider, err := svc.CreateProvider(req)
	require.NoError(t, err)

	assert.NotEmpty(t, provider.ID)
	assert.Equal(t, ProviderOpenAI, provider.Provider)
	assert.Equal(t, "My OpenAI", provider.Name)
	assert.True(t, provider.Enabled)
}

func TestService_GetProvider(t *testing.T) {
	svc := setupTestService(t)

	// 创建一个 provider
	req := CreateProviderRequest{
		Provider: ProviderDeepSeek,
		Name:     "DeepSeek Test",
		APIKey:   "test-key",
	}
	created, err := svc.CreateProvider(req)
	require.NoError(t, err)

	// 获取这个 provider
	provider, err := svc.GetProvider(created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, provider.ID)
	assert.Equal(t, "DeepSeek Test", provider.Name)
}

func TestService_GetProvider_NotFound(t *testing.T) {
	svc := setupTestService(t)

	_, err := svc.GetProvider("nonexistent")
	assert.Error(t, err)
}

func TestService_UpdateProvider(t *testing.T) {
	svc := setupTestService(t)

	// 创建一个 provider
	req := CreateProviderRequest{
		Provider: ProviderOpenAI,
		Name:     "Original Name",
		APIKey:   "original-key",
	}
	created, err := svc.CreateProvider(req)
	require.NoError(t, err)

	// 更新
	enabled := false
	updated, err := svc.UpdateProvider(created.ID, UpdateProviderRequest{
		Name:    "Updated Name",
		Enabled: &enabled,
	})
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", updated.Name)
	assert.False(t, updated.Enabled)
}

func TestService_DeleteProvider(t *testing.T) {
	svc := setupTestService(t)

	// 创建一个 provider
	req := CreateProviderRequest{
		Provider: ProviderGroq,
		Name:     "To Delete",
	}
	created, err := svc.CreateProvider(req)
	require.NoError(t, err)

	// 删除
	err = svc.DeleteProvider(created.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = svc.GetProvider(created.ID)
	assert.Error(t, err)
}

func TestService_CreateProvider_WithCustomBaseURL(t *testing.T) {
	svc := setupTestService(t)

	req := CreateProviderRequest{
		Provider: ProviderOllama,
		Name:     "Custom Ollama",
		BaseURL:  "http://192.168.1.100:11434",
	}

	provider, err := svc.CreateProvider(req)
	require.NoError(t, err)

	assert.Equal(t, "http://192.168.1.100:11434", provider.BaseURL)
}

// ----- Conversation 测试 -----

func TestService_CreateConversation(t *testing.T) {
	svc := setupTestService(t)

	req := CreateConversationRequest{
		Title:    "Test Conversation",
		ModelID:  "llama2",
		Provider: ProviderOllama,
	}

	conv, err := svc.CreateConversation(req)
	require.NoError(t, err)

	assert.NotEmpty(t, conv.ID)
	assert.Equal(t, "Test Conversation", conv.Title)
	assert.Equal(t, "llama2", conv.ModelID)
	assert.Empty(t, conv.Messages)
}

func TestService_GetConversation(t *testing.T) {
	svc := setupTestService(t)

	// 创建对话
	req := CreateConversationRequest{
		Title:    "My Chat",
		ModelID:  "gpt-4",
		Provider: ProviderOpenAI,
	}
	created, err := svc.CreateConversation(req)
	require.NoError(t, err)

	// 获取对话
	conv, err := svc.GetConversation(created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, conv.ID)
	assert.Equal(t, "My Chat", conv.Title)
}

func TestService_GetConversation_NotFound(t *testing.T) {
	svc := setupTestService(t)

	_, err := svc.GetConversation("nonexistent")
	assert.Error(t, err)
}

func TestService_GetConversations(t *testing.T) {
	svc := setupTestService(t)

	// 创建多个对话
	for i := 0; i < 3; i++ {
		_, err := svc.CreateConversation(CreateConversationRequest{
			Title:   "Conv " + string(rune('A'+i)),
			ModelID: "model",
		})
		require.NoError(t, err)
	}

	convs := svc.GetConversations()
	assert.Len(t, convs, 3)
}

func TestService_UpdateConversation(t *testing.T) {
	svc := setupTestService(t)

	// 创建对话
	created, err := svc.CreateConversation(CreateConversationRequest{
		Title: "Original",
	})
	require.NoError(t, err)

	// 更新标题
	updated, err := svc.UpdateConversation(created.ID, UpdateConversationRequest{
		Title: "Updated Title",
	})
	require.NoError(t, err)

	assert.Equal(t, "Updated Title", updated.Title)
}

func TestService_DeleteConversation(t *testing.T) {
	svc := setupTestService(t)

	// 创建对话
	created, err := svc.CreateConversation(CreateConversationRequest{
		Title: "To Delete",
	})
	require.NoError(t, err)

	// 删除
	err = svc.DeleteConversation(created.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = svc.GetConversation(created.ID)
	assert.Error(t, err)
}

// ----- AddMessage 测试 -----

func TestService_AddMessage(t *testing.T) {
	svc := setupTestService(t)

	// 创建对话
	created, err := svc.CreateConversation(CreateConversationRequest{
		Title: "Chat",
	})
	require.NoError(t, err)

	// 添加消息
	msg := Message{
		ID:      "msg-1",
		Role:    "user",
		Content: "Hello AI!",
	}
	err = svc.AddMessage(created.ID, msg)
	require.NoError(t, err)

	// 验证消息已添加
	conv, err := svc.GetConversation(created.ID)
	require.NoError(t, err)
	assert.Len(t, conv.Messages, 1)
	assert.Equal(t, "Hello AI!", conv.Messages[0].Content)
}

// ----- 持久化测试 -----

func TestService_ProvidersPersistence(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()

	// 创建服务并添加 provider
	svc1 := NewService(logger, tempDir)
	_, err := svc1.CreateProvider(CreateProviderRequest{
		Provider: ProviderClaude,
		Name:     "Persistent Claude",
		APIKey:   "claude-key",
	})
	require.NoError(t, err)

	// 验证文件被创建
	providersFile := filepath.Join(tempDir, "providers.json")
	assert.FileExists(t, providersFile)

	// 创建新服务实例，应该加载已保存的数据
	svc2 := NewService(logger, tempDir)
	providers := svc2.GetProviders()

	// 应该包含 deepseek、ollama-local 和新创建的 claude
	assert.GreaterOrEqual(t, len(providers), 3)

	var foundClaude bool
	for _, p := range providers {
		if p.Name == "Persistent Claude" {
			foundClaude = true
			break
		}
	}
	assert.True(t, foundClaude, "claude provider should be persisted")
}

func TestService_ConversationsPersistence(t *testing.T) {
	tempDir := t.TempDir()
	logger := zap.NewNop()

	// 创建服务并添加对话
	svc1 := NewService(logger, tempDir)
	_, err := svc1.CreateConversation(CreateConversationRequest{
		Title:   "Persisted Chat",
		ModelID: "model-x",
	})
	require.NoError(t, err)

	// 立即刷新到磁盘
	svc1.FlushSave()

	// 验证文件被创建
	convsFile := filepath.Join(tempDir, "conversations.json")
	_, err = os.Stat(convsFile)
	require.NoError(t, err)

	// 创建新服务实例
	svc2 := NewService(logger, tempDir)
	convs := svc2.GetConversations()

	var foundChat bool
	for _, c := range convs {
		if c.Title == "Persisted Chat" {
			foundChat = true
			break
		}
	}
	assert.True(t, foundChat, "conversation should be persisted")
}

// ----- APIKey 隐藏测试 -----

func TestService_GetProviders_HidesAPIKey(t *testing.T) {
	svc := setupTestService(t)

	// 创建带 API Key 的 provider
	_, err := svc.CreateProvider(CreateProviderRequest{
		Provider: ProviderOpenAI,
		Name:     "OpenAI with Key",
		APIKey:   "sk-secret-key-12345",
	})
	require.NoError(t, err)

	// 获取列表时 API Key 应该被隐藏
	providers := svc.GetProviders()
	for _, p := range providers {
		if p.Name == "OpenAI with Key" {
			assert.Equal(t, "***", p.APIKey)
			break
		}
	}
}

// ----- 默认 BaseURL 测试 -----

func TestService_CreateProvider_DefaultBaseURLs(t *testing.T) {
	svc := setupTestService(t)

	tests := []struct {
		provider Provider
		expected string
	}{
		{ProviderOpenAI, "https://api.openai.com"},
		{ProviderClaude, "https://api.anthropic.com"},
		{ProviderDeepSeek, "https://api.deepseek.com"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			provider, err := svc.CreateProvider(CreateProviderRequest{
				Provider: tt.provider,
				Name:     "Test " + string(tt.provider),
				APIKey:   "key",
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, provider.BaseURL)
		})
	}
}

// ----- SearchConversations 测试 -----

func TestService_SearchConversations_MatchesTitle(t *testing.T) {
	svc := setupTestService(t)

	_, err := svc.CreateConversation(CreateConversationRequest{Title: "关于 Docker 的问题"})
	require.NoError(t, err)
	_, err = svc.CreateConversation(CreateConversationRequest{Title: "AI 助手介绍"})
	require.NoError(t, err)
	_, err = svc.CreateConversation(CreateConversationRequest{Title: "文件管理教程"})
	require.NoError(t, err)

	results := svc.SearchConversations("docker")
	require.Len(t, results, 1)
	assert.Contains(t, strings.ToLower(results[0].Title), "docker")
}

func TestService_SearchConversations_EmptyQueryReturnsAll(t *testing.T) {
	svc := setupTestService(t)

	for i := 0; i < 3; i++ {
		_, err := svc.CreateConversation(CreateConversationRequest{Title: fmt.Sprintf("Conv %d", i+1)})
		require.NoError(t, err)
	}

	results := svc.SearchConversations("")
	assert.Len(t, results, 3)
}

func TestService_SearchConversations_NoMatch(t *testing.T) {
	svc := setupTestService(t)

	_, err := svc.CreateConversation(CreateConversationRequest{Title: "Hello World"})
	require.NoError(t, err)

	results := svc.SearchConversations("nonexistent")
	assert.Empty(t, results)
}

// ----- trimMessages 测试 -----

func TestTrimMessages_BelowLimit(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "Hi"},
		{Role: "assistant", Content: "Hello"},
	}
	trimmed := trimMessages(msgs, 20)
	assert.Equal(t, msgs, trimmed)
}

func TestTrimMessages_ExceedsLimit(t *testing.T) {
	msgs := make([]Message, 30)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = Message{Role: "user", Content: "question"}
		} else {
			msgs[i] = Message{Role: "assistant", Content: "answer"}
		}
	}

	trimmed := trimMessages(msgs, 20)
	assert.LessOrEqual(t, len(trimmed), 20)
	// 第一条消息必须是 user
	if len(trimmed) > 0 {
		assert.Equal(t, "user", trimmed[0].Role)
	}
}

func TestTrimMessages_StartsWithUser(t *testing.T) {
	// 取最后 3 条：assistant("4"), user("5")。为了保证以 user 开头，应跳过 assistant("4")
	msgs := []Message{
		{Role: "user", Content: "1"},
		{Role: "assistant", Content: "2"},
		{Role: "user", Content: "3"},
		{Role: "assistant", Content: "4"},
		{Role: "user", Content: "5"},
	}
	// 取最后 3 条: assistant("4"), user("5") ... 但为了以 user 开头应跳过 assistant("4")
	trimmed := trimMessages(msgs, 3)
	assert.Greater(t, len(trimmed), 0)
	assert.Equal(t, "user", trimmed[0].Role)
}
