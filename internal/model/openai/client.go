// Package openai 提供一个 OpenAI 兼容的 Chat Completions 客户端实现。
//
// 设计参考了示例文件 example/openai/client.go：
//   - 协议层使用 OpenAI /v1/chat/completions；
//   - 实现 agent.Model 接口，供 Agent 直接使用；
//   - 把协议细节（messages 里 system 放最前、tool_calls 的结构等）封装在此包，
//     对 Agent 层完全透明。
//
// 该客户端被称为 "OpenAI 兼容" 的原因：只要某个服务实现了相同的 HTTP 协议
// （Azure OpenAI、Ollama 的 /v1、以及国内多数大模型的 OpenAI 兼容端点），
// 均可通过这个客户端接入。
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tinycode/internal/agent"
)

// Client 是 OpenAI 兼容 API 的 HTTP 客户端。
// 它对外只暴露一个 Complete 方法（通过实现 agent.Model 接口）。
type Client struct {
	baseURL    string       // 形如 "https://api.openai.com/v1"（末尾斜杠会被去掉）
	apiKey     string       // 可为空以支持某些无认证的内部端点
	model      string       // 例如 "gpt-4o-mini"
	httpClient *http.Client // HTTP 客户端（可自定义 Timeout / Transport）
	temp       float64      // 采样温度，默认 0.1 以追求稳定输出
	observer   Observer     // 模型交互观测器；默认 NopObserver，不产生开销
}

// Option 采用 Functional Options 模式配置客户端，与 agent 包保持一致的风格。
type Option func(*Client)

// WithHTTPClient 使用自定义的 http.Client（例如接入代理、开启 TLS 校验等）。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTemperature 调整采样温度。值越低越稳定，值越高越有创造性。
func WithTemperature(t float64) Option {
	return func(c *Client) { c.temp = t }
}

// WithObserver 注入一个观测器，用于记录每次与模型的 HTTP 交互。
// 传 nil 等价于关闭观测。
func WithObserver(o Observer) Option {
	return func(c *Client) {
		if o == nil {
			c.observer = NopObserver{}
			return
		}
		c.observer = o
	}
}

// NewClient 创建一个 OpenAI 兼容客户端。
// baseURL 示例：https://api.openai.com/v1；model 示例：gpt-4o-mini。
func NewClient(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		temp:       0.1,
		observer:   NopObserver{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Name 返回模型名称。
func (c *Client) Name() string { return c.model }

// Complete 向 Chat Completions 接口发起一次请求，并把响应转换回 agent.CompletionResponse。
//
// 流程：
//  1. 将内部 Message / ToolDefinition 转换为 OpenAI 所需的 payload；
//  2. 发送 HTTP POST；
//  3. 解析响应中的 choices[0].message，还原 content / tool_calls；
//  4. tool_calls.arguments 是字符串化 JSON，需再解析一次。
func (c *Client) Complete(ctx context.Context, req agent.CompletionRequest) (agent.CompletionResponse, error) {
	// 1) 组装请求体
	payload := map[string]any{
		"model":       c.model,
		"temperature": c.temp,
		"messages":    toOpenAIMessages(req.SystemPrompt, req.Messages),
	}
	// 仅当存在工具时才附带 tools / tool_choice，避免空数组被某些兼容实现拒绝
	if tools := toOpenAITools(req.Tools); len(tools) > 0 {
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("marshal payload: %w", err)
	}

	// 2) 构造 HTTP 请求
	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body),
	)
	if err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// 观测钩子：请求发出前记录完整 URL 与 payload，以便出错时回放。
	c.observer.OnRequest(httpReq.URL.String(), httpReq.Header, body)
	start := time.Now()

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.observer.OnError(err, time.Since(start))
		return agent.CompletionResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.observer.OnError(err, time.Since(start))
		return agent.CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	// 观测钩子：无论状态码都记录响应体，非 2xx 也能看到上游返回的详细错误。
	c.observer.OnResponse(resp.StatusCode, respBody, time.Since(start))

	if resp.StatusCode >= 300 {
		return agent.CompletionResponse{}, fmt.Errorf(
			"openai api error: status=%s body=%s", resp.Status, truncate(string(respBody), 1024),
		)
	}

	// 3) 解析响应
	var decoded struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("decode response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return agent.CompletionResponse{}, fmt.Errorf("openai api: no choices")
	}

	choice := decoded.Choices[0]
	msg := choice.Message

	// 4) 还原工具调用
	toolCalls := make([]agent.ToolCall, 0, len(msg.ToolCalls))
	for _, call := range msg.ToolCalls {
		args, perr := parseArguments(call.Function.Arguments)
		if perr != nil {
			// 解析失败时保留原始字节，让工具在 Execute 时再做一次校验
			args = call.Function.Arguments
		}
		toolCalls = append(toolCalls, agent.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: args,
		})
	}

	return agent.CompletionResponse{
		Message: agent.Message{
			Role:      agent.RoleAssistant,
			Content:   msg.Content,
			ToolCalls: toolCalls,
		},
		ToolCalls: toolCalls,
		Stop:      choice.FinishReason,
	}, nil
}

// 编译期断言：Client 必须满足 agent.Model 接口。
var _ agent.Model = (*Client)(nil)

// ------------------------ 下面是协议转换辅助函数 ------------------------

// toOpenAIMessages 将内部消息转换为 OpenAI 要求的 messages 数组。
//
// 关键点：
//   - system 必须作为 messages 数组的第一项；
//   - assistant 若包含 tool_calls，需要同时携带 tool_calls 字段；
//   - tool 消息需要 tool_call_id（与 assistant 的某次 tool_calls[i].id 关联）；
//   - tool 消息的 name 字段在最新 OpenAI 协议中非必需，这里不填；
//   - 空 content 的规范化（见下方 normalizeContent 注释）。
func toOpenAIMessages(systemPrompt string, messages []agent.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages)+1)
	if systemPrompt != "" {
		result = append(result, map[string]any{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	for _, m := range messages {
		item := map[string]any{
			"role":    m.Role,
			"content": normalizeContent(m),
		}
		if m.ToolCallID != "" {
			item["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			item["tool_calls"] = toOpenAIToolCalls(m.ToolCalls)
		}
		result = append(result, item)
	}
	return result
}

// normalizeContent 规范化消息的 content 字段，避免被部分 OpenAI 兼容代理拒绝：
//   - assistant 且只发起 tool_calls（content 为空）时，按官方协议给出 null，
//     否则部分代理（尤其国内一些 one-api 风格网关）会以 500 Internal Server Error 报错。
//   - tool 消息 content 不允许为空字符串（同样会触发上游 500），
//     当工具无输出时填一个占位符，让模型知道命令执行成功但没有输出。
//   - 其它情况原样返回。
func normalizeContent(m agent.Message) any {
	if m.Role == agent.RoleAssistant && m.Content == "" && len(m.ToolCalls) > 0 {
		return nil // 将序列化为 JSON null
	}
	if m.Role == agent.RoleTool && m.Content == "" {
		return "(工具无输出)"
	}
	return m.Content
}

// toOpenAIToolCalls 将内部 ToolCall 转换为 OpenAI 的 tool_calls 数组。
// 注意 arguments 在协议里是"字符串形式的 JSON"，所以要 string(...) 一次。
func toOpenAIToolCalls(calls []agent.ToolCall) []map[string]any {
	result := make([]map[string]any, 0, len(calls))
	for _, c := range calls {
		result = append(result, map[string]any{
			"id":   c.ID,
			"type": "function",
			"function": map[string]any{
				"name":      c.Name,
				"arguments": string(c.Arguments),
			},
		})
	}
	return result
}

// toOpenAITools 将内部工具清单转换为 OpenAI 的 tools 数组。
// 当工具列表为空时返回 nil，调用方可据此决定是否写入 tools / tool_choice 字段。
func toOpenAITools(defs []agent.ToolDefinition) []map[string]any {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]map[string]any, 0, len(defs))
	for _, d := range defs {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        d.Name,
				"description": d.Description,
				"parameters":  json.RawMessage(d.Parameters),
			},
		})
	}
	return tools
}

// parseArguments 规整工具参数：
//   - 若是字符串化的 JSON（`"{\"a\":1}"`），则先反字符串再解析为对象；
//   - 若已是 JSON 对象，原样返回。
//
// OpenAI 官方的 arguments 是"字符串形式的 JSON"，但为了让工具方使用更自然，
// 这里统一把它规范化成 JSON 对象字节流。
func parseArguments(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	// 以双引号起始 → 字符串
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return raw, err
		}
		if strings.TrimSpace(s) == "" {
			return json.RawMessage(`{}`), nil
		}
		var obj any
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return raw, err
		}
		return json.Marshal(obj)
	}
	return raw, nil
}

// truncate 仅用于错误日志，避免把超大响应体原样打印。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
