// Package ai Bot 沙箱桥接测试
package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeSandboxExecutor 模拟 bot 沙箱执行器
type fakeSandboxExecutor struct {
	execScript   string
	execLanguage string
	runSkillName string
	runSkillArgs string
	execErr      error
}

func (f *fakeSandboxExecutor) ExecInBot(ctx context.Context, botID, language, script string, timeoutSec int) (*BotExecResult, error) {
	if f.execErr != nil {
		return nil, f.execErr
	}
	f.execLanguage = language
	f.execScript = script
	return &BotExecResult{ExitCode: 0, Output: "done", Truncated: false}, nil
}

func (f *fakeSandboxExecutor) RunSkill(ctx context.Context, botID, skillName, argsJSON string) (string, error) {
	f.runSkillName = skillName
	f.runSkillArgs = argsJSON
	return "skill output", nil
}

// fakeSkillProvider 模拟技能提供方
type fakeSkillProvider struct {
	skills map[string]string
}

func (f *fakeSkillProvider) ListSkills() []SkillMeta {
	metas := make([]SkillMeta, 0, len(f.skills))
	for name := range f.skills {
		metas = append(metas, SkillMeta{Name: name, Description: name + " 描述"})
	}
	return metas
}

func (f *fakeSkillProvider) GetSkillContent(name string) (string, error) {
	content, ok := f.skills[name]
	if !ok {
		return "", errors.New("技能不存在")
	}
	return content, nil
}

func TestBotToolsDefinitions(t *testing.T) {
	tools := botToolsDefinitions()
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Function.Name] = true
		assert.Equal(t, "function", tool.Type)
		assert.NotEmpty(t, tool.Function.Description)
	}
	assert.True(t, names["list_skills"])
	assert.True(t, names["get_skill"])
	assert.True(t, names["execute_script"])
	assert.True(t, names["run_skill"])
}

func TestExecuteBotTool(t *testing.T) {
	svc := setupTestService(t)
	executor := &fakeSandboxExecutor{}
	provider := &fakeSkillProvider{skills: map[string]string{"hello": "问好技能"}}

	// 未注入桥接 → 所有工具回落到本地技能（返回 false）
	result, isBot := svc.executeBotTool(context.Background(), "list_skills", "{}", "bot-1")
	assert.False(t, isBot)
	assert.Empty(t, result)

	svc.setBotBridge(executor, nil, provider, nil)

	// 无 BotID → 不路由
	_, isBot = svc.executeBotTool(context.Background(), "execute_script", "{}", "")
	assert.False(t, isBot)

	ctx := context.Background()

	// list_skills
	result, isBot = svc.executeBotTool(ctx, "list_skills", "{}", "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, "hello")

	// get_skill
	result, isBot = svc.executeBotTool(ctx, "get_skill", `{"name":"hello"}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, "问好技能")

	// get_skill 未知名
	result, isBot = svc.executeBotTool(ctx, "get_skill", `{"name":"nope"}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, "技能不存在")

	// execute_script
	result, isBot = svc.executeBotTool(ctx, "execute_script", `{"language":"python","script":"print(1)","timeout":999}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, `"output":"done"`)
	assert.Equal(t, "python", executor.execLanguage)
	assert.Equal(t, "print(1)", executor.execScript)

	// run_skill
	result, isBot = svc.executeBotTool(ctx, "run_skill", `{"name":"hello","args_json":"{\"a\":1}"}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, "skill output")
	assert.Equal(t, "hello", executor.runSkillName)
	assert.Equal(t, `{"a":1}`, executor.runSkillArgs)

	// 沙箱执行失败
	executor.execErr = errors.New("沙箱未运行")
	result, isBot = svc.executeBotTool(ctx, "execute_script", `{"language":"bash","script":"ls"}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, "沙箱未运行")

	// 非 Bot 工具名 → false
	_, isBot = svc.executeBotTool(ctx, "get_system_info", "{}", "bot-1")
	assert.False(t, isBot)
}

func TestExecuteBotTool_Memory(t *testing.T) {
	svc := setupTestService(t)
	memory := &fakeMemoryProvider{}
	ctx := context.Background()

	// 未注入 memory → 工具仍属 Bot 工具但报告不可用
	result, isBot := svc.executeBotTool(ctx, "save_memory", `{"key":"k","content":"v"}`, "bot-1")
	assert.True(t, isBot)
	assert.Contains(t, result, "memory provider unavailable")

	svc.setBotBridge(nil, nil, nil, memory)

	// save_memory
	result, isBot = svc.executeBotTool(ctx, "save_memory", `{"key":"偏好","content":"简洁回复"}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, `"key":"偏好"`)
	assert.Equal(t, "偏好", memory.savedKey)
	assert.Equal(t, "简洁回复", memory.savedContent)

	// 缺参数
	result, isBot = svc.executeBotTool(ctx, "save_memory", `{"key":"k"}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, "missing")

	// search_memory
	result, isBot = svc.executeBotTool(ctx, "search_memory", `{"query":"偏好"}`, "bot-1")
	require.True(t, isBot)
	assert.Contains(t, result, "简洁回复")
	assert.Equal(t, "偏好", memory.searchQuery)

	// 无 BotID → 不路由
	_, isBot = svc.executeBotTool(ctx, "search_memory", `{"query":"x"}`, "")
	assert.False(t, isBot)
}

// fakeMemoryProvider 模拟记忆提供方
type fakeMemoryProvider struct {
	savedKey      string
	savedContent  string
	searchQuery   string
	items         []MemoryItem
	saveErr       error
}

func (f *fakeMemoryProvider) SaveMemory(botID, key, content string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedKey, f.savedContent = key, content
	f.items = append(f.items, MemoryItem{Key: key, Content: content, UpdatedAt: "now"})
	return nil
}

func (f *fakeMemoryProvider) SearchMemory(botID, query string, limit int) ([]MemoryItem, error) {
	f.searchQuery = query
	return f.items, nil
}

func TestBuildBotSystemPrompt(t *testing.T) {
	svc := setupTestService(t)

	// 无技能提供方
	bot := &BotInfo{ID: "b1", Name: "小明", Description: "写日报"}
	prompt := svc.buildBotSystemPrompt(bot)
	assert.Contains(t, prompt, "小明")
	assert.Contains(t, prompt, "写日报")
	assert.Contains(t, prompt, "execute_script")

	// 有技能提供方 → 注入技能索引
	svc.setBotBridge(nil, nil, &fakeSkillProvider{skills: map[string]string{"hello": "x"}}, nil)
	prompt = svc.buildBotSystemPrompt(bot)
	assert.Contains(t, prompt, "hello")
	assert.Contains(t, prompt, "get_skill")
}

func TestGateway_ResolveBotForSession(t *testing.T) {
	logger := zap.NewNop()
	tempDir := t.TempDir()
	gs := NewGatewayService(logger, tempDir, setupTestService(t), nil)

	// 未注入 resolver → nil
	msg := IncomingMessage{Platform: "telegram", ChatID: "c1", UserID: "u1"}
	session := gs.getOrCreateSession(msg)
	assert.Nil(t, gs.resolveBotForSession(msg, session))

	// 注入 resolver（命中）
	gs.setBotBridge(&fakeBotResolver{bot: &BotInfo{ID: "b1", Name: "小明"}})
	bot := gs.resolveBotForSession(msg, session)
	require.NotNil(t, bot)
	assert.Equal(t, "b1", session.BotID)

	// 绑定变化 → 清空历史
	session.Messages = []GatewayMessage{{Role: "user", Content: "hi"}}
	msg2 := IncomingMessage{Platform: "telegram", ChatID: "c2", UserID: "u2"}
	bot2 := gs.resolveBotForSession(msg2, session)
	require.NotNil(t, bot2)
	assert.Empty(t, session.Messages)
	assert.Equal(t, "b2", session.BotID)

	// 未命中 → 解绑
	gs.setBotBridge(&fakeBotResolver{bot: nil})
	assert.Nil(t, gs.resolveBotForSession(msg2, session))
	assert.Empty(t, session.BotID)
}

// fakeBotResolver 模拟渠道解析
type fakeBotResolver struct {
	bot   *BotInfo
	calls int
}

func (f *fakeBotResolver) ResolveBotByChannel(platform, chatID, userID string) *BotInfo {
	f.calls++
	if chatID == "c2" && f.bot != nil {
		return &BotInfo{ID: "b2", Name: "第二个"}
	}
	return f.bot
}
