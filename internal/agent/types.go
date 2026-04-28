// Package agent 定义 Agent 的核心类型、接口与循环实现。
//
// 本文件聚焦"中性的消息模型"：它不与任何具体大模型 API（OpenAI / Anthropic）强耦合，
// 而是以一种通用的方式描述对话内容，由各 Model 实现负责在边界处做格式转换。
package agent

import "encoding/json"

// 会话中常用的角色常量，保持和 OpenAI chat/completions 协议一致，
// 同时便于 Shell / 其它工具消息的统一表达。
const (
	RoleSystem    = "system"    // 系统提示（只作为 SystemPrompt 传入，不存入消息历史）
	RoleUser      = "user"      // 用户消息
	RoleAssistant = "assistant" // 模型回复（可能包含 tool_calls）
	RoleTool      = "tool"      // 工具执行结果
)

// Message 表示一次对话中的单条消息。
//
// 字段设计原则：
//   - 所有与"外部协议"相关的字段（tool_call_id / tool_calls）都用 omitempty，
//     以便序列化时不会污染普通文本消息。
//   - Content 只保存纯文本内容；工具调用的结构化信息走 ToolCalls / ToolCallID。
type Message struct {
	// Role 是消息角色，取值见 RoleXxx 常量。
	Role string `json:"role"`

	// Content 是消息文本内容。对于 assistant 来说可能为空（当它只发起了工具调用时）。
	Content string `json:"content"`

	// ToolCallID 仅当 Role == RoleTool 时填充，指向 assistant 发起的某次 ToolCall.ID，
	// 用于在上下文中把工具结果与请求正确关联。
	ToolCallID string `json:"tool_call_id,omitempty"`

	// ToolCalls 仅当 Role == RoleAssistant 且模型请求调用工具时填充。
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall 描述一次模型发起的工具调用请求。
//
// 注意：Arguments 是原始 JSON（json.RawMessage），由工具实现自己反序列化到具体参数结构，
// 从而避免在框架层做任何形状假设。
type ToolCall struct {
	ID        string          `json:"id"`        // 工具调用的唯一 ID，由模型生成
	Name      string          `json:"name"`      // 目标工具名
	Arguments json.RawMessage `json:"arguments"` // 工具参数（JSON 对象）
}

// ToolDefinition 是发送给模型的"可用工具清单"元数据。
//
// Parameters 遵循 JSON Schema 规范，在 OpenAI 的 tools 字段里会被原样放入
// function.parameters 字段。
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// CompletionRequest 是 Agent 向 Model 发起一次补全的请求。
//
// 说明：SystemPrompt 独立出来传入，Model 实现负责在必要时将其转换为
// 协议要求的 system 消息（OpenAI 即是如此）。
type CompletionRequest struct {
	SystemPrompt string           // 系统提示，可为空
	Messages     []Message        // 对话历史（不包含 system 消息）
	Tools        []ToolDefinition // 可用工具清单
}

// CompletionResponse 是 Model 返回的单次补全结果。
//
// ToolCalls 与 Message.ToolCalls 保持同值；单独放在外面是为了方便调用方
// 只通过 resp.ToolCalls 判断是否需要执行工具，而不必深入 Message。
type CompletionResponse struct {
	Message   Message
	ToolCalls []ToolCall
	Stop      string // "stop" 表示结束；"tool_calls" 表示模型想调用工具
}
