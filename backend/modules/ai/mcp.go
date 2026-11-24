// Package ai MCP（Model Context Protocol）兼容层
//
// admin 在控制面配置 MCP 服务器（可信基础设施），工具以
// `mcp_<server>_<tool>` 命名桥接进 Bot 工具列表，调用经本管理器转发。
// 支持两种传输：
//   - http：Streamable HTTP（POST JSON-RPC，响应 application/json 或 SSE）
//   - stdio：子进程 stdin/stdout 换行分隔 JSON-RPC
//
// 配置持久化在 dataDir/mcp.json（与 gateway.json 同层，避免 DB 迁移）。
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
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// MCPServerConfig MCP 服务器配置
type MCPServerConfig struct {
	Name        string            `json:"name"`              // 服务名（[a-z0-9-]，进工具前缀）
	Description string            `json:"description,omitempty"`
	Transport   string            `json:"transport"`         // http / stdio
	URL         string            `json:"url,omitempty"`     // http：JSON-RPC 端点
	Headers     map[string]string `json:"headers,omitempty"` // http：附加头（如 Authorization）
	Command     string            `json:"command,omitempty"` // stdio：可执行文件
	Args        []string          `json:"args,omitempty"`    // stdio：参数
	Env         map[string]string `json:"env,omitempty"`     // stdio：环境变量
	Enabled     bool              `json:"enabled"`
}

// MCPToolInfo MCP 工具（tools/list 结果）
type MCPToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// mcpToolsCacheEntry 工具列表缓存（tools/list 结果 + 获取时间）
type mcpToolsCacheEntry struct {
	tools     []MCPToolInfo
	fetchedAt time.Time
}

// mcpStdioProc stdio 传输的常驻子进程
type mcpStdioProc struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	mu      sync.Mutex // 串行化请求/响应
	nextID  int64
}

// MCPManager MCP 服务器注册表与客户端
type MCPManager struct {
	logger  *zap.Logger
	dataDir string
	mu      sync.RWMutex
	servers map[string]*MCPServerConfig

	client      *http.Client
	sessionMu   sync.Mutex
	httpSession map[string]string // http 服务器 → Mcp-Session-Id
	procMu      sync.Mutex
	procs       map[string]*mcpStdioProc
	toolsCache  map[string]mcpToolsCacheEntry
}

// NewMCPManager 创建 MCP 管理器并加载配置
func NewMCPManager(logger *zap.Logger, dataDir string) *MCPManager {
	m := &MCPManager{
		logger:      logger,
		dataDir:     dataDir,
		servers:     make(map[string]*MCPServerConfig),
		client:      &http.Client{Timeout: 60 * time.Second},
		httpSession: make(map[string]string),
		procs:       make(map[string]*mcpStdioProc),
		toolsCache:  make(map[string]mcpToolsCacheEntry),
	}
	m.loadConfig()
	return m
}

func (m *MCPManager) configFile() string {
	return filepath.Join(m.dataDir, "mcp.json")
}

func (m *MCPManager) loadConfig() {
	data, err := os.ReadFile(m.configFile())
	if err != nil {
		return
	}
	var servers []*MCPServerConfig
	if err := json.Unmarshal(data, &servers); err != nil {
		m.logger.Error("MCP 配置加载失败", zap.Error(err))
		return
	}
	for _, s := range servers {
		if s.Name != "" {
			m.servers[s.Name] = s
		}
	}
}

func (m *MCPManager) saveConfig() error {
	var servers []*MCPServerConfig
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configFile(), data, 0644)
}

// mcpNameRe 服务名规则（工具名前缀安全）
var mcpNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// List 服务器列表（配置全量，不含运行期状态）
func (m *MCPManager) List() []MCPServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]MCPServerConfig, 0, len(m.servers))
	for _, s := range m.servers {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// Upsert 新增/更新服务器配置
func (m *MCPManager) Upsert(cfg MCPServerConfig) error {
	cfg.Name = strings.ToLower(strings.TrimSpace(cfg.Name))
	if !mcpNameRe.MatchString(cfg.Name) {
		return fmt.Errorf("服务名仅允许小写字母、数字与连字符（%q）", cfg.Name)
	}
	if cfg.Transport != "http" && cfg.Transport != "stdio" {
		return fmt.Errorf("transport 仅支持 http / stdio")
	}
	if cfg.Transport == "http" && cfg.URL == "" {
		return fmt.Errorf("http 传输需要 url")
	}
	if cfg.Transport == "stdio" && cfg.Command == "" {
		return fmt.Errorf("stdio 传输需要 command")
	}

	m.mu.Lock()
	m.servers[cfg.Name] = &cfg
	m.mu.Unlock()

	// 配置变更：丢弃运行期会话与缓存
	m.resetRuntime(cfg.Name)
	return m.saveConfig()
}

// Delete 删除服务器
func (m *MCPManager) Delete(name string) error {
	m.mu.Lock()
	if _, ok := m.servers[name]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("MCP 服务器 %s 不存在", name)
	}
	delete(m.servers, name)
	m.mu.Unlock()

	m.resetRuntime(name)
	return m.saveConfig()
}

// resetRuntime 丢弃会话/子进程/工具缓存
func (m *MCPManager) resetRuntime(name string) {
	m.sessionMu.Lock()
	delete(m.httpSession, name)
	m.sessionMu.Unlock()

	m.procMu.Lock()
	if p, ok := m.procs[name]; ok {
		delete(m.procs, name)
		go p.kill()
	}
	m.procMu.Unlock()

	m.mu.Lock()
	delete(m.toolsCache, name)
	m.mu.Unlock()
}

// Get 获取单个服务器配置
func (m *MCPManager) Get(name string) (*MCPServerConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[name]
	return s, ok
}

// ==================== 工具桥接 ====================

// MCPToolPrefix 桥接工具名前缀
const MCPToolPrefix = "mcp_"

// BridgedToolName 桥接工具名：mcp_<server>_<tool>
func BridgedToolName(server, tool string) string {
	return MCPToolPrefix + server + "_" + tool
}

// SplitBridgedToolName 解析桥接工具名（返回 server、tool）
func SplitBridgedToolName(name string) (string, string, bool) {
	if !strings.HasPrefix(name, MCPToolPrefix) {
		return "", "", false
	}
	rest := name[len(MCPToolPrefix):]
	idx := strings.Index(rest, "_")
	if idx <= 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+1:], true
}

// ToolDefinitions 已启用服务器的桥接工具定义（供 LLM 工具列表）
func (m *MCPManager) ToolDefinitions(ctx context.Context) []ToolDefinition {
	tools := m.listAllTools(ctx)
	defs := make([]ToolDefinition, 0, len(tools))
	for _, t := range tools {
		params := t.InputSchema
		if params == nil {
			params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		defs = append(defs, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        t.BridgedName,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return defs
}

// bridgedTool 桥接后的工具（带全名）
type bridgedTool struct {
	BridgedName  string
	Server       string
	Tool         string
	Description  string
	InputSchema  map[string]interface{}
}

// listAllTools 汇总已启用服务器的工具（失败的服务器跳过并记日志）
func (m *MCPManager) listAllTools(ctx context.Context) []bridgedTool {
	m.mu.RLock()
	enabled := make([]*MCPServerConfig, 0)
	for _, s := range m.servers {
		if s.Enabled {
			enabled = append(enabled, s)
		}
	}
	m.mu.RUnlock()

	var result []bridgedTool
	for _, s := range enabled {
		tools, err := m.ListServerTools(ctx, s.Name)
		if err != nil {
			m.logger.Warn("MCP tools/list 失败（跳过）", zap.String("server", s.Name), zap.Error(err))
			continue
		}
		for _, t := range tools {
			desc := t.Description
			if s.Description != "" {
				desc = fmt.Sprintf("[%s] %s", s.Description, desc)
			}
			result = append(result, bridgedTool{
				BridgedName: BridgedToolName(s.Name, t.Name),
				Server:      s.Name,
				Tool:        t.Name,
				Description: desc,
				InputSchema: t.InputSchema,
			})
		}
	}
	return result
}

// ListServerTools 某服务器的工具（5 分钟缓存）
func (m *MCPManager) ListServerTools(ctx context.Context, name string) ([]MCPToolInfo, error) {
	m.mu.RLock()
	cached, ok := m.toolsCache[name]
	m.mu.RUnlock()
	if ok && time.Since(cached.fetchedAt) < 5*time.Minute {
		return cached.tools, nil
	}

	cfg, ok := m.Get(name)
	if !ok {
		return nil, fmt.Errorf("MCP 服务器 %s 不存在", name)
	}

	var tools []MCPToolInfo
	var err error
	if cfg.Transport == "stdio" {
		tools, err = m.stdioListTools(ctx, cfg)
	} else {
		tools, err = m.httpListTools(ctx, cfg)
	}
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.toolsCache[name] = mcpToolsCacheEntry{tools: tools, fetchedAt: time.Now()}
	m.mu.Unlock()
	return tools, nil
}

// CallTool 调用桥接工具（name=mcp_<server>_<tool>）
func (m *MCPManager) CallTool(ctx context.Context, name, argsJSON string) (string, error) {
	server, tool, ok := SplitBridgedToolName(name)
	if !ok {
		return "", fmt.Errorf("无效的 MCP 工具名 %q", name)
	}
	cfg, ok := m.Get(server)
	if !ok {
		return "", fmt.Errorf("MCP 服务器 %s 不存在", server)
	}
	if !cfg.Enabled {
		return "", fmt.Errorf("MCP 服务器 %s 已停用", server)
	}

	var args map[string]interface{}
	if strings.TrimSpace(argsJSON) != "" {
		if err := jsonUnmarshalLoose(argsJSON, &args); err != nil {
			args = nil
		}
	}

	var raw json.RawMessage
	var err error
	if cfg.Transport == "stdio" {
		raw, err = m.stdioCall(ctx, cfg, tool, args)
	} else {
		raw, err = m.httpCall(ctx, cfg, tool, args)
	}
	if err != nil {
		return "", err
	}
	return formatMCPToolResult(raw), nil
}

// formatMCPToolResult 把 tools/call result 转成给 LLM 的 JSON 字符串
func formatMCPToolResult(raw json.RawMessage) string {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		// 非 JSON：原样返回
		return string(raw)
	}
	var texts []string
	for _, c := range result.Content {
		if c.Text != "" {
			texts = append(texts, c.Text)
		}
	}
	out := map[string]interface{}{"success": !result.IsError}
	if len(texts) > 0 {
		out["content"] = strings.Join(texts, "\n")
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// TestConnection 连接测试（initialize + tools/list）
func (m *MCPManager) TestConnection(ctx context.Context, name string) ([]MCPToolInfo, error) {
	m.resetRuntime(name)
	return m.ListServerTools(ctx, name)
}

// ==================== JSON-RPC 公共 ====================

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

var mcpRPCID int64

func nextMCPID() int64 {
	return atomic.AddInt64(&mcpRPCID, 1)
}

func mcpInitializeParams() map[string]interface{} {
	return map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "rde-mcp-client", "version": "1.0"},
	}
}

// ==================== HTTP（Streamable HTTP）客户端 ====================

// httpRPC 向 HTTP MCP 服务器发一次 JSON-RPC（自动带会话头；解析 JSON 或 SSE 响应）
func (m *MCPManager) httpRPC(ctx context.Context, cfg *MCPServerConfig, method string, params interface{}, withSession bool) (*jsonRPCResponse, error) {
	req := jsonRPCRequest{JSONRPC: "2.0", ID: nextMCPID(), Method: method, Params: params}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range cfg.Headers {
		httpReq.Header.Set(k, v)
	}
	if withSession {
		m.sessionMu.Lock()
		sid := m.httpSession[cfg.Name]
		m.sessionMu.Unlock()
		if sid != "" {
			httpReq.Header.Set("Mcp-Session-Id", sid)
		}
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("MCP HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 会话 id（initialize 响应返回，后续请求带上）
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		m.sessionMu.Lock()
		m.httpSession[cfg.Name] = sid
		m.sessionMu.Unlock()
	}

	if resp.StatusCode == 202 {
		// 通知被接受（无响应体）
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		// 会话失效：丢弃缓存，下次重新 initialize
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			m.sessionMu.Lock()
			delete(m.httpSession, cfg.Name)
			m.sessionMu.Unlock()
		}
		return nil, fmt.Errorf("MCP HTTP 状态 %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// application/json → 单个响应对象；text/event-stream → data: 行取匹配 id 的那个
	ct := resp.Header.Get("Content-Type")
	var candidates [][]byte
	if strings.Contains(ct, "text/event-stream") {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				candidates = append(candidates, []byte(strings.TrimSpace(line[len("data:"):])) )
			}
		}
	} else {
		candidates = append(candidates, data)
	}

	for _, c := range candidates {
		var r jsonRPCResponse
		if err := json.Unmarshal(c, &r); err != nil || r.ID != req.ID {
			continue
		}
		if r.Error != nil {
			return &r, fmt.Errorf("MCP 错误 %d: %s", r.Error.Code, r.Error.Message)
		}
		return &r, nil
	}
	return nil, fmt.Errorf("MCP HTTP 响应中没有匹配的 JSON-RPC 结果")
}

// httpInitialize 握手（initialize + notifications/initialized）
func (m *MCPManager) httpInitialize(ctx context.Context, cfg *MCPServerConfig) error {
	resp, err := m.httpRPC(ctx, cfg, "initialize", mcpInitializeParams(), false)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("initialize 无响应")
	}
	// initialized 通知（202 Accepted 属正常）
	_, _ = m.httpRPC(ctx, cfg, "notifications/initialized", nil, true)
	return nil
}

func (m *MCPManager) httpListTools(ctx context.Context, cfg *MCPServerConfig) ([]MCPToolInfo, error) {
	m.sessionMu.Lock()
	_, hasSession := m.httpSession[cfg.Name]
	m.sessionMu.Unlock()
	if !hasSession {
		if err := m.httpInitialize(ctx, cfg); err != nil {
			return nil, fmt.Errorf("initialize 失败: %w", err)
		}
	}
	resp, err := m.httpRPC(ctx, cfg, "tools/list", map[string]interface{}{}, true)
	if err != nil {
		// 会话可能失效，重试一次
		m.sessionMu.Lock()
		delete(m.httpSession, cfg.Name)
		m.sessionMu.Unlock()
		if err2 := m.httpInitialize(ctx, cfg); err2 != nil {
			return nil, fmt.Errorf("tools/list: %v（重连失败: %v）", err, err2)
		}
		resp, err = m.httpRPC(ctx, cfg, "tools/list", map[string]interface{}{}, true)
		if err != nil {
			return nil, err
		}
	}
	if resp == nil {
		return nil, fmt.Errorf("tools/list 无响应")
	}
	return parseMCPTools(resp.Result), nil
}

func (m *MCPManager) httpCall(ctx context.Context, cfg *MCPServerConfig, tool string, args map[string]interface{}) (json.RawMessage, error) {
	m.sessionMu.Lock()
	_, hasSession := m.httpSession[cfg.Name]
	m.sessionMu.Unlock()
	if !hasSession {
		if err := m.httpInitialize(ctx, cfg); err != nil {
			return nil, fmt.Errorf("initialize 失败: %w", err)
		}
	}
	params := map[string]interface{}{"name": tool}
	if args != nil {
		params["arguments"] = args
	}
	resp, err := m.httpRPC(ctx, cfg, "tools/call", params, true)
	if err != nil {
		// 会话可能失效，重试一次
		m.sessionMu.Lock()
		delete(m.httpSession, cfg.Name)
		m.sessionMu.Unlock()
		if err2 := m.httpInitialize(ctx, cfg); err2 != nil {
			return nil, fmt.Errorf("tools/call: %v（重连失败: %v）", err, err2)
		}
		resp, err = m.httpRPC(ctx, cfg, "tools/call", params, true)
		if err != nil {
			return nil, err
		}
	}
	if resp == nil {
		return nil, fmt.Errorf("tools/call 无响应")
	}
	return resp.Result, nil
}

// parseMCPTools 解析 tools/list result
func parseMCPTools(result json.RawMessage) []MCPToolInfo {
	var r struct {
		Tools []MCPToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return nil
	}
	return r.Tools
}

// ==================== stdio 客户端 ====================

// getStdioProc 获取（或启动并初始化）stdio 子进程
func (m *MCPManager) getStdioProc(ctx context.Context, cfg *MCPServerConfig) (*mcpStdioProc, error) {
	m.procMu.Lock()
	defer m.procMu.Unlock()
	if p, ok := m.procs[cfg.Name]; ok {
		return p, nil
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 MCP 进程失败: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // 工具结果可能较大
	p := &mcpStdioProc{cmd: cmd, stdin: stdin, scanner: scanner}

	// initialize 握手
	if _, err := p.rpc(ctx, "initialize", mcpInitializeParams()); err != nil {
		p.kill()
		return nil, fmt.Errorf("initialize 失败: %w", err)
	}
	if err := p.notify("notifications/initialized"); err != nil {
		p.kill()
		return nil, err
	}

	m.procs[cfg.Name] = p
	return p, nil
}

// rpc 发一次请求并等待匹配 id 的响应
func (p *mcpStdioProc) rpc(ctx context.Context, method string, params interface{}) (*jsonRPCResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	req := jsonRPCRequest{JSONRPC: "2.0", ID: nextMCPID(), Method: method, Params: params}
	body, _ := json.Marshal(req)
	if _, err := p.stdin.Write(append(body, '\n')); err != nil {
		return nil, fmt.Errorf("MCP stdin 写入失败: %w", err)
	}

	// 读到匹配 id 的响应（跳过服务器主动推送的通知/请求）
	for p.scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		line := bytes.TrimSpace(p.scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var r jsonRPCResponse
		if err := json.Unmarshal(line, &r); err != nil || r.ID != req.ID {
			continue
		}
		if r.Error != nil {
			return &r, fmt.Errorf("MCP 错误 %d: %s", r.Error.Code, r.Error.Message)
		}
		return &r, nil
	}
	if err := p.scanner.Err(); err != nil {
		return nil, fmt.Errorf("MCP stdout 读取失败: %w", err)
	}
	return nil, fmt.Errorf("MCP 进程输出流关闭（进程退出）")
}

// notify 发通知（无响应）
func (p *mcpStdioProc) notify(method string) error {
	n := jsonRPCNotification{JSONRPC: "2.0", Method: method}
	body, _ := json.Marshal(n)
	_, err := p.stdin.Write(append(body, '\n'))
	return err
}

// kill 结束子进程（幂等）
func (p *mcpStdioProc) kill() {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

func (m *MCPManager) stdioListTools(ctx context.Context, cfg *MCPServerConfig) ([]MCPToolInfo, error) {
	p, err := m.getStdioProc(ctx, cfg)
	if err != nil {
		// 进程可能已死，清理后重试一次
		m.procMu.Lock()
		delete(m.procs, cfg.Name)
		m.procMu.Unlock()
		p, err = m.getStdioProc(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}
	resp, err := p.rpc(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		m.dropStdioProc(cfg.Name)
		return nil, err
	}
	return parseMCPTools(resp.Result), nil
}

func (m *MCPManager) stdioCall(ctx context.Context, cfg *MCPServerConfig, tool string, args map[string]interface{}) (json.RawMessage, error) {
	p, err := m.getStdioProc(ctx, cfg)
	if err != nil {
		m.procMu.Lock()
		delete(m.procs, cfg.Name)
		m.procMu.Unlock()
		p, err = m.getStdioProc(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}
	params := map[string]interface{}{"name": tool}
	if args != nil {
		params["arguments"] = args
	}
	resp, err := p.rpc(ctx, "tools/call", params)
	if err != nil {
		m.dropStdioProc(cfg.Name)
		return nil, err
	}
	return resp.Result, nil
}

func (m *MCPManager) dropStdioProc(name string) {
	m.procMu.Lock()
	if p, ok := m.procs[name]; ok {
		delete(m.procs, name)
		go p.kill()
	}
	m.procMu.Unlock()
}
