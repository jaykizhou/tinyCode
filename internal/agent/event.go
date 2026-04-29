package agent

import "encoding/json"

// EventKind 枚举了 Agent 在一次 RunLoop 中可能对外发出的结构化事件类型。
//
// 设计取舍：
//   - 事件是"扁平的值对象"，不含 UI 语义，UI 层自行决定如何渲染；
//   - 预留 EventAssistantDelta 用于后续接入流式 token，而不改变外层协议；
//   - 事件与 WithLogger 并存：logger 走人类可读文本，sink 走结构化事件。
type EventKind string

const (
	// EventIterStart 表示新一轮主循环开始（i 从 1 起计）。
	EventIterStart EventKind = "iter.start"

	// EventAssistantDelta 为流式 token 预留，当前同步实现不会发出。
	EventAssistantDelta EventKind = "assistant.delta"

	// EventAssistantReply 表示本轮 assistant 消息已完整可见（可能只是工具调用，无正文）。
	EventAssistantReply EventKind = "assistant.reply"

	// EventToolCall 表示即将执行某次工具调用。
	EventToolCall EventKind = "tool.call"

	// EventToolResult 表示某次工具调用已返回结果（成功或失败均算结果）。
	EventToolResult EventKind = "tool.result"

	// EventError 表示出现不可恢复的错误（比如 model.Complete 失败）。
	EventError EventKind = "error"

	// EventDone 表示整个 RunLoop 已结束（正常返回或因错误终止）。
	EventDone EventKind = "done"
)

// Event 是 Agent 对外广播的结构化事件。
//
// 字段说明：
//   - 所有字段均可选，具体由 Kind 决定语义；
//   - Payload 承载文本内容（回复正文 / 工具输出 / 错误消息）；
//   - Args 承载工具入参原始 JSON，方便 UI 做摘要展示。
type Event struct {
	Kind       EventKind
	Iter       int             // 当前迭代序号（1 起），与 EventIterStart 对齐
	ToolName   string          // EventToolCall / EventToolResult 使用
	ToolCallID string          // EventToolCall / EventToolResult 使用
	Payload    string          // 通用文本载荷
	Args       json.RawMessage // 工具入参原始 JSON（可选）
}

// EventSink 是外部（UI/日志/监控）订阅 Agent 事件的入口。
//
// 实现者须保证 Emit 是并发安全且不会阻塞 Agent 主循环——
// 建议内部使用带缓冲 channel + 非阻塞 select 丢弃慢消费者。
type EventSink interface {
	Emit(Event)
}

// noopSink 是默认的空实现，用于未注入 sink 时避免到处 nil 判断。
type noopSink struct{}

func (noopSink) Emit(Event) {}

// EventSinkFunc 让普通函数可直接当作 EventSink 使用。
type EventSinkFunc func(Event)

// Emit 实现 EventSink 接口。
func (f EventSinkFunc) Emit(e Event) { f(e) }
