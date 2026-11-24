// Package ai — Bot 沙箱桥接接口
//
// 依赖方向约定：ai 模块不 import bot；bot 模块在 Init 时通过
// Module.SetBotBridge 注入以下接口的实现（拓扑序保证 ai 先于 bot 初始化）。
// P3：IM 网关消息按 Bot.Channels 路由到对应 Bot，
// 工具调用（execute_script / run_skill 等）在该 Bot 的沙箱容器内执行。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// BotInfo Bot 的摘要信息（bot 模块 → ai 模块的传输结构，避免直接依赖）
type BotInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"` // 职责描述，作为 system prompt 的一部分
	Model       string `json:"model"`
	ProviderID  string `json:"provider_id"`
}

// BotExecResult 沙箱执行结果
type BotExecResult struct {
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

// SkillMeta 技能元数据（SKILL.md frontmatter）
type SkillMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SandboxExecutor 在 Bot 沙箱内执行代码/技能（由 bot 模块注入）
type SandboxExecutor interface {
	// ExecInBot 在指定 Bot 的沙箱容器内执行脚本（语言：bash/python）
	ExecInBot(ctx context.Context, botID, language, script string, timeoutSec int) (*BotExecResult, error)
	// RunSkill 执行技能的预置入口脚本（SKILL.md 同目录 scripts/run.*）
	RunSkill(ctx context.Context, botID, skillName, argsJSON string) (string, error)
}

// SkillProvider 技能文件（SKILL.md）提供方（由 bot 模块注入）
type SkillProvider interface {
	// ListSkills 技能索引（name + description，渐进式披露的第一级）
	ListSkills() []SkillMeta
	// GetSkillContent 返回 SKILL.md 的正文指令（渐进式披露的第二级）
	GetSkillContent(name string) (string, error)
}

// BotResolver 按 IM 渠道信息解析绑定的 Bot（由 bot 模块注入）
type BotResolver interface {
	// ResolveBotByChannel 按 platform + chatID/userID 匹配 Bot.Channels
	// 未匹配返回 nil（走默认 AI 助手）
	ResolveBotByChannel(platform, chatID, userID string) *BotInfo
}

// MemoryItem 记忆条目
type MemoryItem struct {
	Key       string `json:"key"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}

// MemoryProvider Bot 长期记忆存取（由 bot 模块注入，SQLite 存储）
type MemoryProvider interface {
	// SaveMemory 保存/覆盖一条记忆（同 key 覆盖）
	SaveMemory(botID, key, content string) error
	// SearchMemory 按关键词检索记忆（无关键词返回最近条目）
	SearchMemory(botID, query string, limit int) ([]MemoryItem, error)
}

// botBridge ai 模块持有的桥接实现集合
type botBridge struct {
	executor SandboxExecutor
	resolver BotResolver
	skills   SkillProvider
	memory   MemoryProvider
}

// SetBotBridge 注入 bot 模块的桥接实现（bot.Module Init 调用）
func (m *Module) SetBotBridge(executor SandboxExecutor, resolver BotResolver, skills SkillProvider, memory MemoryProvider) {
	m.service.setBotBridge(executor, resolver, skills, memory)
	m.gateway.setBotBridge(resolver)
	m.ctx.Logger.Info("bot bridge injected into ai module")
}

// setBotBridge 设置 service 的桥接引用
func (s *Service) setBotBridge(executor SandboxExecutor, resolver BotResolver, skills SkillProvider, memory MemoryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.botExecutor = executor
	s.botResolver = resolver
	s.botSkills = skills
	s.botMemory = memory
}

// setBotBridge 设置网关的 Bot 解析器
func (gs *GatewayService) setBotBridge(resolver BotResolver) {
	gs.botResolver = resolver
}

// RunBotTask 以指定 Bot 的身份执行一次性任务（定时调度/立即执行共用）
// prompt 作为用户消息进入带工具的对话循环，返回最终回复文本。
func (m *Module) RunBotTask(ctx context.Context, bot *BotInfo, prompt string) (string, error) {
	messages := []GatewayMessage{
		{Role: "user", Content: "【定时任务】" + prompt},
	}
	return m.service.ChatWithToolsForBot(ctx, messages, bot)
}

// SendToChannel 向指定 IM 渠道推送文本（Bot 定时任务结果推送用）
// 按平台取网关适配器发送；平台未启用返回错误。
func (m *Module) SendToChannel(ctx context.Context, platform, chatID, userID, text string) error {
	adapter := m.gateway.GetAdapter(PlatformType(platform))
	if adapter == nil {
		return fmt.Errorf("平台 %s 未启用", platform)
	}
	msg := OutgoingMessage{
		Platform: PlatformType(platform),
		ChatID:   chatID,
		UserID:   userID,
		Text:     text,
	}
	return adapter.SendMessage(ctx, msg)
}

// botToolsDefinitions Bot 沙箱工具定义（存在 Bot 上下文时附加到工具列表）
func botToolsDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{Type: "function", Function: FunctionDef{
			Name:        "list_skills",
			Description: "列出 Bot 可用的技能清单（技能名与简述）。先调用本工具了解有哪些技能，再按需读取技能详情。",
			Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		}},
		{Type: "function", Function: FunctionDef{
			Name:        "get_skill",
			Description: "读取指定技能的完整指令内容（SKILL.md 正文），据此生成可在 Bot 沙箱中执行的脚本。",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "技能名（来自 list_skills）"},
			}, "required": []string{"name"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name:        "execute_script",
			Description: "在当前 Bot 的沙箱容器内执行一段 bash 或 python 脚本。沙箱是隔离的 Linux 桌面环境，脚本只影响沙箱内部。",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"language": map[string]interface{}{"type": "string", "description": "脚本语言：bash 或 python", "enum": []string{"bash", "python"}},
				"script":   map[string]interface{}{"type": "string", "description": "完整脚本内容"},
				"timeout":  map[string]interface{}{"type": "integer", "description": "超时秒数，默认 60，上限 300"},
			}, "required": []string{"language", "script"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name:        "run_skill",
			Description: "执行技能自带的预置入口脚本（技能目录下 scripts/run.sh 或 scripts/run.py），适合已封装好的固定动作。",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"name":      map[string]interface{}{"type": "string", "description": "技能名"},
				"args_json": map[string]interface{}{"type": "string", "description": "传给脚本的参数（JSON 字符串，脚本从环境变量 SKILL_ARGS 读取）"},
			}, "required": []string{"name"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name:        "save_memory",
			Description: "把重要信息存入长期记忆（跨对话保留，如任务结论、用户偏好、待办）。同 key 再次保存会覆盖旧值。",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"key":     map[string]interface{}{"type": "string", "description": "记忆的键（简短主题词，如「日报格式」「负责人」）"},
				"content": map[string]interface{}{"type": "string", "description": "记忆内容"},
			}, "required": []string{"key", "content"}},
		}},
		{Type: "function", Function: FunctionDef{
			Name:        "search_memory",
			Description: "检索长期记忆。开始任务前先查相关记忆，避免重复询问已知信息。",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string", "description": "关键词（空则返回最近的记忆）"},
			}},
		}},
	}
}

// executeBotTool 执行 Bot 沙箱/记忆工具；返回 false 表示不是 Bot 工具（走原有 SkillsService）
func (s *Service) executeBotTool(ctx context.Context, name, argsJSON, botID string) (string, bool) {
	if botID == "" {
		return "", false
	}

	s.mu.RLock()
	executor := s.botExecutor
	skills := s.botSkills
	memory := s.botMemory
	s.mu.RUnlock()

	var args struct {
		Name     string `json:"name"`
		Language string `json:"language"`
		Script   string `json:"script"`
		Timeout  int    `json:"timeout"`
		ArgsJSON string `json:"args_json"`
		Key      string `json:"key"`
		Content  string `json:"content"`
		Query    string `json:"query"`
	}
	_ = jsonUnmarshalLoose(argsJSON, &args)

	switch name {
	case "save_memory":
		if memory == nil {
			return `{"success": false, "error": "memory provider unavailable"}`, true
		}
		if args.Key == "" || args.Content == "" {
			return `{"success": false, "error": "missing key or content"}`, true
		}
		if err := memory.SaveMemory(botID, args.Key, args.Content); err != nil {
			return marshalToolResult(map[string]interface{}{"success": false, "error": err.Error()}), true
		}
		return marshalToolResult(map[string]interface{}{"success": true, "key": args.Key}), true

	case "search_memory":
		if memory == nil {
			return `{"success": false, "error": "memory provider unavailable"}`, true
		}
		limit := 8
		items, err := memory.SearchMemory(botID, args.Query, limit)
		if err != nil {
			return marshalToolResult(map[string]interface{}{"success": false, "error": err.Error()}), true
		}
		return marshalToolResult(map[string]interface{}{"success": true, "data": items}), true
	}

	// 以下工具依赖沙箱执行器
	if executor == nil {
		return "", false
	}

	switch name {
	case "list_skills":
		if skills == nil {
			return `{"success": false, "error": "skill provider unavailable"}`, true
		}
		list := skills.ListSkills()
		return marshalToolResult(map[string]interface{}{"success": true, "data": list}), true

	case "get_skill":
		if skills == nil {
			return `{"success": false, "error": "skill provider unavailable"}`, true
		}
		if args.Name == "" {
			return `{"success": false, "error": "missing skill name"}`, true
		}
		content, err := skills.GetSkillContent(args.Name)
		if err != nil {
			return marshalToolResult(map[string]interface{}{"success": false, "error": err.Error()}), true
		}
		return marshalToolResult(map[string]interface{}{"success": true, "data": content}), true

	case "execute_script":
		if args.Language == "" || args.Script == "" {
			return `{"success": false, "error": "missing language or script"}`, true
		}
		if args.Timeout <= 0 || args.Timeout > 300 {
			args.Timeout = 60
		}
		result, err := executor.ExecInBot(ctx, botID, args.Language, args.Script, args.Timeout)
		if err != nil {
			return marshalToolResult(map[string]interface{}{"success": false, "error": err.Error()}), true
		}
		return marshalToolResult(map[string]interface{}{
			"success": result.ExitCode == 0,
			"exit_code": result.ExitCode,
			"output": result.Output,
			"truncated": result.Truncated,
			"error": func() string {
				if result.ExitCode != 0 {
					return "脚本退出码非 0"
				}
				return ""
			}(),
		}), true

	case "run_skill":
		if args.Name == "" {
			return `{"success": false, "error": "missing skill name"}`, true
		}
		out, err := executor.RunSkill(ctx, botID, args.Name, args.ArgsJSON)
		if err != nil {
			return marshalToolResult(map[string]interface{}{"success": false, "error": err.Error()}), true
		}
		return marshalToolResult(map[string]interface{}{"success": true, "output": out}), true
	}

	return "", false
}

// jsonUnmarshalLoose 宽松解析工具参数 JSON（解析失败保持零值，不中断工具循环）
func jsonUnmarshalLoose(data string, v interface{}) error {
	if data == "" {
		return nil
	}
	return json.Unmarshal([]byte(data), v)
}

// marshalToolResult 工具结果序列化（失败时返回安全占位）
func marshalToolResult(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"success": false, "error": "marshal tool result failed"}`
	}
	return string(data)
}
