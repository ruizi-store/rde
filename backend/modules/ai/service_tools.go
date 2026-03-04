// Package ai — 流式 Function Calling 支持
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// streamWithToolCalls 流式输出 + Function Calling 组合
// 最多 maxToolRounds 轮工具调用，每轮先非流式请求 LLM 检测工具调用，
// 执行工具后把结果写回来，最终流式输出回答。
func (s *Service) streamWithToolCalls(provider *ProviderConfig, model string, req ChatRequest, stream chan<- StreamChunk) error {
	const maxToolRounds = 5

	// 转换消息格式
	toolMessages := make([]ChatMessageWithTools, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		toolMessages = append(toolMessages, ChatMessageWithTools{Role: "system", Content: req.SystemPrompt})
	}
	for _, msg := range req.Messages {
		toolMessages = append(toolMessages, ChatMessageWithTools{Role: msg.Role, Content: msg.Content})
	}

	tools := s.skills.GetToolDefinitions()

	for round := 0; round < maxToolRounds; round++ {
		toolCalls, content, err := s.callWithTools(provider, model, toolMessages, tools)
		if err != nil {
			return err
		}

		// 没有工具调用 → 直接流式输出
		if len(toolCalls) == 0 {
			if round == 0 {
				// 首轮无工具调用，走普通流式
				return s.streamOpenAI(provider, req, stream)
			}
			// 非首轮，content 即最终结果
			if content != "" {
				stream <- StreamChunk{ID: "final", Delta: content}
			}
			stream <- StreamChunk{ID: "done", FinishReason: "stop"}
			return nil
		}

		// 向前端推送工具调用提示
		stream <- StreamChunk{
			ID:    "thinking-" + uuid.New().String(),
			Delta: fmt.Sprintf("\n\n🔧 *正在调用技能: %s...*\n\n", toolCalls[0].Function.Name),
		}

		// 将 assistant 的消息（含 tool_calls）加入历史
		assistantMsg := ChatMessageWithTools{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		}
		toolMessages = append(toolMessages, assistantMsg)

		// 执行每个工具调用
		for _, tc := range toolCalls {
			result := s.skills.ExecuteToolCall(tc.Function.Name, tc.Function.Arguments)
			toolMessages = append(toolMessages, ChatMessageWithTools{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
			s.logger.Info("Tool executed",
				zap.String("tool", tc.Function.Name),
				zap.Int("result_len", len(result)))
		}
	}

	// 超过最大轮数，做最终流式输出
	return s.streamFinalResponse(provider, model, toolMessages, stream)
}

// callWithTools 非流式调用 LLM，检测工具调用
func (s *Service) callWithTools(provider *ProviderConfig, model string, messages []ChatMessageWithTools, tools []ToolDefinition) ([]ToolCall, string, error) {
	reqBody := OpenAIChatRequestWithTools{
		Model:       model,
		Messages:    messages,
		Tools:       tools,
		Stream:      false,
		MaxTokens:   s.config.MaxTokens,
		Temperature: s.config.Temperature,
	}

	data, _ := json.Marshal(reqBody)
	endpoint := provider.BaseURL + "/v1/chat/completions"

	httpReq, _ := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("%s 返回错误: %d - %s", provider.Provider, resp.StatusCode, string(bodyBytes))
	}

	var result OpenAIChatResponseWithTools
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, "", nil
	}

	choice := result.Choices[0]
	return choice.Message.ToolCalls, choice.Message.Content, nil
}

// streamFinalResponse 工具调用后最终流式输出
func (s *Service) streamFinalResponse(provider *ProviderConfig, model string, messages []ChatMessageWithTools, stream chan<- StreamChunk) error {
	reqBody := OpenAIChatRequestWithTools{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	data, _ := json.Marshal(reqBody)
	endpoint := provider.BaseURL + "/v1/chat/completions"

	httpReq, _ := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s 返回错误: %d - %s", provider.Provider, resp.StatusCode, string(bodyBytes))
	}

	return s.parseOpenAISSEStream(resp.Body, stream)
}

// parseOpenAISSEStream 通用 OpenAI SSE 流解析
func (s *Service) parseOpenAISSEStream(body io.Reader, stream chan<- StreamChunk) error {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)

	for {
		n, err := body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)

			// 按 \n\n 拆分 SSE 事件
			for {
				idx := bytes.Index(buf, []byte("\n\n"))
				if idx < 0 {
					break
				}
				event := string(buf[:idx])
				buf = buf[idx+2:]

				for _, line := range splitLines(event) {
					if len(line) > 6 && line[:6] == "data: " {
						data := line[6:]
						if data == "[DONE]" {
							stream <- StreamChunk{FinishReason: "stop"}
							return nil
						}
						var chunk OpenAIChatResponseWithTools
						if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
							content := chunk.Choices[0].Delta.Content
							if content != "" {
								stream <- StreamChunk{ID: chunk.ID, Delta: content}
							}
							if chunk.Choices[0].FinishReason == "stop" {
								stream <- StreamChunk{FinishReason: "stop"}
								return nil
							}
						}
					}
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
