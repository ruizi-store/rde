// IM 适配器（Telegram/飞书/钉钉/MCP）交互式审批与桥接测试
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeApprovalByIDManager 同时实现 ApprovalManager 与 ApprovalByIDDecider
type fakeApprovalByIDManager struct {
	approvalID uint
	approve    bool
	byUser     string
	decided    int
	err        error
}

func (f *fakeApprovalByIDManager) RequestApproval(ctx context.Context, botID, action, reason string, timeoutSec int) (bool, string, error) {
	return true, "已批准", nil
}
func (f *fakeApprovalByIDManager) DecideApproval(botID string, approvalID uint, approve bool, byUser string) (string, error) {
	return "已处理", nil
}
func (f *fakeApprovalByIDManager) DecideApprovalByID(approvalID uint, approve bool, byUser string) (string, error) {
	f.decided++
	f.approvalID, f.approve, f.byUser = approvalID, approve, byUser
	if f.err != nil {
		return "", f.err
	}
	return fmt.Sprintf("✅ 审批 #%d 已批准", approvalID), nil
}

func TestParseApprovalNotification(t *testing.T) {
	id, ok := parseApprovalNotification("🔔 审批请求 #12（客服Bot）\n操作: 删除文件\n同意回复 /approve 12")
	require.True(t, ok)
	assert.Equal(t, uint(12), id)

	_, ok = parseApprovalNotification("普通消息")
	assert.False(t, ok)

	_, ok = parseApprovalNotification("🔔 审批请求 #abc")
	assert.False(t, ok)
}

func TestGateway_ApprovalWebLink(t *testing.T) {
	logger := zap.NewNop()
	gs := NewGatewayService(logger, t.TempDir(), nil, nil)

	assert.Equal(t, "", gs.ApprovalWebLink(1))

	gs.config.PublicBaseURL = "https://rde.example.com/"
	assert.Equal(t, "https://rde.example.com/?approval=7", gs.ApprovalWebLink(7))
}

func TestGateway_HandleApprovalButton(t *testing.T) {
	logger := zap.NewNop()
	gs := NewGatewayService(logger, t.TempDir(), nil, nil)

	// 未注入审批管理
	_, err := gs.HandleApprovalButton(3, true, "u")
	assert.Error(t, err)

	fake := &fakeApprovalByIDManager{}
	gs.setApprovalManager(fake)

	reply, err := gs.HandleApprovalButton(9, false, "feishu:ou_123")
	require.NoError(t, err)
	assert.Contains(t, reply, "审批")
	assert.Equal(t, uint(9), fake.approvalID)
	assert.False(t, fake.approve)
	assert.Equal(t, "feishu:ou_123", fake.byUser)
}

// ==================== Telegram ====================

func TestTelegramApprovalKeyboardAndCallback(t *testing.T) {
	// mock Telegram API
	calls := make(chan string, 8) // 收到的方法名
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"result":{}}`)
	}))
	defer srv.Close()

	logger := zap.NewNop()
	gs := NewGatewayService(logger, t.TempDir(), nil, nil)
	fake := &fakeApprovalByIDManager{}
	gs.setApprovalManager(fake)

	ta := NewTelegramAdapterWithTimeout(logger, &TelegramConfig{
		BotToken: "TEST:TOKEN", APIBase: srv.URL,
	}, gs, 5)

	// 1) 审批通知附 inline keyboard
	kb := ta.approvalKeyboard(12)
	require.Len(t, kb.InlineKeyboard, 1)
	require.Len(t, kb.InlineKeyboard[0], 2)
	assert.Equal(t, "approve:12", kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "deny:12", kb.InlineKeyboard[0][1].CallbackData)

	// 2) callback_query → 决定审批 + answer + 编辑消息
	update := TelegramUpdate{
		UpdateID: 1,
		CallbackQuery: &TelegramCallbackQuery{
			ID:   "cbq-1",
			From: &TelegramUser{ID: 42, FirstName: "Tester"},
			Message: &TelegramMessage{
				MessageID: 100,
				Chat:      &TelegramChat{ID: 555},
			},
			Data: "approve:12",
		},
	}
	ta.dispatchUpdate(&update)

	// 等待异步处理
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fake.decided > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, 1, fake.decided)
	assert.Equal(t, uint(12), fake.approvalID)
	assert.True(t, fake.approve)
	assert.Equal(t, "telegram:42", fake.byUser)

	// answerCallbackQuery + editMessageText 两个调用
	assert.Equal(t, "/botTEST:TOKEN/answerCallbackQuery", <-calls)
	assert.Equal(t, "/botTEST:TOKEN/editMessageText", <-calls)
}

func TestTelegramHandleWebhookCallbackQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"result":{}}`)
	}))
	defer srv.Close()

	logger := zap.NewNop()
	gs := NewGatewayService(logger, t.TempDir(), nil, nil)
	fake := &fakeApprovalByIDManager{}
	gs.setApprovalManager(fake)

	ta := NewTelegramAdapterWithTimeout(logger, &TelegramConfig{
		BotToken: "TEST:TOKEN", APIBase: srv.URL,
	}, gs, 5)

	body := `{"update_id":7,"callback_query":{"id":"c1","from":{"id":7},"message":{"message_id":9,"chat":{"id":3}},"data":"deny:4"}}`
	ta.HandleWebhook([]byte(body))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fake.decided > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, 1, fake.decided)
	assert.False(t, fake.approve)
}

// ==================== 飞书 ====================

func TestFeishuHandleCallback(t *testing.T) {
	logger := zap.NewNop()
	gs := NewGatewayService(logger, t.TempDir(), nil, nil)
	fake := &fakeApprovalByIDManager{}
	gs.setApprovalManager(fake)

	fa := NewFeishuAdapter(logger, &FeishuConfig{Enabled: true, AppID: "a", AppSecret: "s"}, gs)

	// 1) url_verification 握手
	result := fa.HandleCallback([]byte(`{"type":"url_verification","challenge":"abc123"}`))
	assert.Equal(t, "abc123", result["challenge"])

	// 2) 校验失败（错误 token；配置 VerificationToken 后）
	fa.config.VerificationToken = "TOK"
	body := `{"schema":"2.0","header":{"event_type":"im.message.receive_v1","token":"WRONG"},"event":{}}`
	result = fa.HandleCallback([]byte(body))
	assert.Equal(t, 403, result["code"])

	// 3) 卡片按钮回调（审批决定）
	cardBody := `{"schema":"2.0","header":{"event_type":"card.action.trigger","token":"TOK"},
	"event":{"operator":{"operator_id":{"open_id":"ou_9"}},"action":{"value":{"approval_id":15,"approve":true}}}}`
	result = fa.HandleCallback([]byte(cardBody))
	assert.Equal(t, 0, result["code"])

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fake.decided > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, 1, fake.decided)
	assert.Equal(t, uint(15), fake.approvalID)
	assert.Equal(t, "feishu:ou_9", fake.byUser)

	// 4) 未知事件 → code 0
	result = fa.HandleCallback([]byte(`{"schema":"2.0","header":{"event_type":"other.event","token":"TOK"},"event":{}}`))
	assert.Equal(t, 0, result["code"])
}

func TestStripFeishuMentions(t *testing.T) {
	assert.Equal(t, "你好", stripFeishuMentions("@_user_1 你好"))
	// @ 后跟随的空白一并剥离（飞书 mention 约定后跟一个空格）
	assert.Equal(t, "hello world", stripFeishuMentions("@_user_2 hello world"))
	assert.Equal(t, "纯文本", stripFeishuMentions("纯文本"))
}

// ==================== 钉钉 ====================

func TestDingtalkStreamFrameHandling(t *testing.T) {
	logger := zap.NewNop()
	gs := NewGatewayService(logger, t.TempDir(), nil, nil)

	// 1) sessionWebhook 解析与缓存
	da := NewDingtalkAdapter(logger, &DingtalkConfig{Enabled: true}, gs)
	msgJSON := json.RawMessage(`{"conversationId":"cid1","conversationType":"2","senderId":"u1","senderStaffId":"staff1","senderNick":"张三","text":{"content":"列出文件"},"sessionWebhook":"https://oapi.example.com/hook","sessionWebhookExpiredTime":0}`)
	da.handleBotMessage(msgJSON)

	da.webhookMu.RLock()
	_, ok := da.webhooks["cid1"]
	da.webhookMu.RUnlock()
	assert.True(t, ok)

	// 2) Web 链接 actionCard（配置 PublicBaseURL 时）
	gs.config.PublicBaseURL = "https://rde.example.com"
	payloadType := "text"
	// 直接验证 resolveSendTarget 与 card 分支逻辑：取审批文本
	cardText := "🔔 审批请求 #8（Bot）\n操作: 删除 /data"
	_, isApproval := parseApprovalNotification(cardText)
	require.True(t, isApproval)
	assert.NotEmpty(t, gs.ApprovalWebLink(8))
	_ = payloadType

	// 3) 加签 URL 构造（不实际发送，仅验证 resolveSendTarget 优先 chatID 直发）
	target, err := da.resolveSendTarget(OutgoingMessage{ChatID: "http://hook.example/x"})
	require.NoError(t, err)
	assert.Equal(t, "http://hook.example/x", target)
}

func TestDingtalkStreamAck(t *testing.T) {
	// WebSocket 帧回执测试：SYSTEM → pong（客户端写回 ack，服务端收到并断言）
	var upgrader = websocket.Upgrader{}
	serverGotAck := make(chan map[string]interface{}, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// 服务器推一帧 SYSTEM
		sysFrame := map[string]interface{}{
			"specVersion": "1.0", "type": "SYSTEM",
			"headers": map[string]interface{}{"contentType": "application/json", "messageId": "m1"},
			"data":    "ping",
		}
		_ = conn.WriteJSON(sysFrame)
		// 读取客户端回执
		var ack map[string]interface{}
		_ = conn.ReadJSON(&ack)
		serverGotAck <- ack
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	logger := zap.NewNop()
	gs := NewGatewayService(logger, t.TempDir(), nil, nil)
	da := NewDingtalkAdapter(logger, &DingtalkConfig{Enabled: true}, gs)

	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	var frame DingtalkStreamFrame
	require.NoError(t, json.Unmarshal(data, &frame))
	assert.Equal(t, "SYSTEM", frame.Type)
	assert.Equal(t, "m1", frame.Headers.MessageID)

	// handleStreamFrame 处理 SYSTEM → 回 pong
	require.NoError(t, da.handleStreamFrame(conn, &frame))

	var ack map[string]interface{}
	select {
	case ack = <-serverGotAck:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive ack in time")
	}
	assert.Equal(t, float64(200), ack["code"])
	headers := ack["headers"].(map[string]interface{})
	assert.Equal(t, "m1", headers["messageId"])
	assert.Equal(t, "pong", ack["data"])
}

// ==================== MCP ====================

// newMockMCPServer 模拟 Streamable HTTP MCP 服务器
func newMockMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	var sessionSet bool
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int64           `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Mcp-Session-Id") != "" || req.Method != "initialize" {
			sessionSet = true
		}
		switch req.Method {
		case "initialize":
			if !sessionSet {
				w.Header().Set("Mcp-Session-Id", "sess-1")
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-03-26","capabilities":{},"serverInfo":{"name":"mock"}}}`, req.ID)
		case "tools/list":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"get_weather","description":"查询天气","inputSchema":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}]}}`, req.ID)
		case "tools/call":
			var p struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			city := p.Arguments["city"]
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"%v 晴 25℃"}]}}`, req.ID, city)
		default:
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32601,"message":"method not found"}}`, req.ID)
		}
	}))
}

func TestMCPManager_HTTPToolsAndCall(t *testing.T) {
	srv := newMockMCPServer(t)
	defer srv.Close()

	logger := zap.NewNop()
	m := NewMCPManager(logger, t.TempDir())
	require.NoError(t, m.Upsert(MCPServerConfig{
		Name: "weather", Transport: "http", URL: srv.URL, Enabled: true,
		Description: "天气服务",
	}))

	// 工具桥接命名
	defs := m.ToolDefinitions(context.Background())
	require.Len(t, defs, 1)
	assert.Equal(t, "mcp_weather_get_weather", defs[0].Function.Name)
	assert.Contains(t, defs[0].Function.Description, "天气服务")

	server, tool, ok := SplitBridgedToolName("mcp_weather_get_weather")
	require.True(t, ok)
	assert.Equal(t, "weather", server)
	assert.Equal(t, "get_weather", tool)

	_, _, ok = SplitBridgedToolName("mcp_badname")
	assert.False(t, ok)

	// 调用
	result, err := m.CallTool(context.Background(), "mcp_weather_get_weather", `{"city":"北京"}`)
	require.NoError(t, err)
	assert.Contains(t, result, "北京 晴 25℃")
	assert.Contains(t, result, `"success":true`)
}

func TestMCPManager_CRUDAndValidation(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	m := NewMCPManager(logger, dir)

	// 非法名
	err := m.Upsert(MCPServerConfig{Name: "Bad Name", Transport: "http", URL: "http://x"})
	assert.Error(t, err)
	// 非法 transport
	err = m.Upsert(MCPServerConfig{Name: "ok-name", Transport: "grpc"})
	assert.Error(t, err)
	// http 缺 url
	err = m.Upsert(MCPServerConfig{Name: "ok-name", Transport: "http"})
	assert.Error(t, err)

	// 合法 upsert + 持久化 + 重新加载
	require.NoError(t, m.Upsert(MCPServerConfig{Name: "tools", Transport: "stdio", Command: "uvx", Args: []string{"some-mcp"}, Enabled: true}))
	require.NoError(t, m.Upsert(MCPServerConfig{Name: "alpha", Transport: "http", URL: "http://x", Enabled: true}))
	list := m.List()
	require.Len(t, list, 2)
	assert.Equal(t, "alpha", list[0].Name) // 排序

	m2 := NewMCPManager(zap.NewNop(), dir) // 从磁盘加载
	require.Len(t, m2.List(), 2)

	// 删除
	require.NoError(t, m2.Delete("alpha"))
	require.Len(t, m2.List(), 1)
	err = m2.Delete("nope")
	assert.Error(t, err)
}

func TestMCPManager_SSEParsing(t *testing.T) {
	// SSE 响应格式解析
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"tools\":[]}}\n\n", req.ID)
	}))
	defer srv.Close()

	logger := zap.NewNop()
	m := NewMCPManager(logger, t.TempDir())
	require.NoError(t, m.Upsert(MCPServerConfig{Name: "sse", Transport: "http", URL: srv.URL, Enabled: true}))

	tools, err := m.ListServerTools(context.Background(), "sse")
	require.NoError(t, err)
	assert.Empty(t, tools)
}
