package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// 默认系统提示词：足够中性、覆盖最常见的"编程助手"场景。
const defaultSystemPrompt = `你是一个专业的编程助手。你可以调用已注册的工具（例如 shell）
来执行命令、读写文件、运行程序。请在必要时主动调用工具，并基于工具结果给出最终回复。
回答请使用简洁、直接的中文。`

// Agent 是整个系统的核心。它本身非常薄：
//   - 对外暴露 RunLoop，接受一句用户输入并返回模型的最终回复；
//   - 对内依赖 Model（大模型）与 Registry（工具注册表）两个协作者；
//   - 内部状态仅有一个只追加的 Conversation。
//
// 这正是 one_loop.md 所说的 "Harness 尽量薄" 的体现。
type Agent struct {
	model         Model
	systemPrompt  string
	registry      *Registry
	conv          *Conversation
	maxIterations int
	log           func(event string, kv ...any)
	sink          EventSink
}

// NewAgent 构造 Agent。除了 WithModel 是必选，其余 Option 均可省略。
func NewAgent(opts ...Option) (*Agent, error) {
	a := &Agent{
		systemPrompt:  defaultSystemPrompt,
		registry:      NewRegistry(),
		conv:          NewConversation(),
		maxIterations: 20,
		log:           func(string, ...any) {}, // 默认空日志，避免 nil 判断
		sink:          noopSink{},              // 默认空 sink，避免 nil 判断
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.model == nil {
		return nil, ErrModelRequired
	}
	return a, nil
}

// Registry 暴露内部工具注册表，主要用于在创建后动态补充工具。
func (a *Agent) Registry() *Registry { return a.registry }

// Conversation 暴露会话（只读用途：导出、打印历史等）。
func (a *Agent) Conversation() *Conversation { return a.conv }

// RunLoop 是 Agent 的核心——对应 one_loop.md 5.5 的"30 行核心循环"。
//
// 流程：
//  1. 把用户输入作为 user 消息追加到会话；
//  2. 循环调用模型；
//  3. 模型没有 tool_calls → 视为最终回复，返回；
//  4. 模型有 tool_calls → 逐个执行，把结果作为 role=tool 消息追加，继续循环；
//  5. 达到最大迭代次数仍未收敛 → 返回 ErrMaxIterations。
func (a *Agent) RunLoop(ctx context.Context, userInput string) (string, error) {
	a.conv.Append(Message{Role: RoleUser, Content: userInput})

	for i := 0; i < a.maxIterations; i++ {
		iter := i + 1
		a.log("loop.iter", "n", iter)
		a.sink.Emit(Event{Kind: EventIterStart, Iter: iter})

		resp, err := a.model.Complete(ctx, CompletionRequest{
			SystemPrompt: a.systemPrompt,
			Messages:     a.conv.Snapshot(),
			Tools:        a.registry.Definitions(),
		})
		if err != nil {
			// 对取消场景做特殊处理：视为"正常终止"，不发 EventError
			if errors.Is(err, context.Canceled) {
				a.sink.Emit(Event{
					Kind:    EventDone,
					Iter:    iter,
					Payload: "context canceled",
				})
				return "", err
			}

			wrapped := fmt.Errorf("model complete (iter %d): %w", iter, err)
			a.sink.Emit(Event{Kind: EventError, Iter: iter, Payload: wrapped.Error()})
			a.sink.Emit(Event{Kind: EventDone, Iter: iter, Payload: wrapped.Error()})
			return "", wrapped
		}

		// 不管有没有 tool_calls，都把模型响应完整追加到历史里，
		// 这是后续工具结果能够正确关联 tool_call_id 的前提。
		a.conv.Append(Message{
			Role:      RoleAssistant,
			Content:   resp.Message.Content,
			ToolCalls: resp.ToolCalls,
		})
		a.sink.Emit(Event{Kind: EventAssistantReply, Iter: iter, Payload: resp.Message.Content})

		// 没有工具调用：模型认为任务完成，退出循环。
		if len(resp.ToolCalls) == 0 {
			a.log("loop.done", "content_len", len(resp.Message.Content))
			a.sink.Emit(Event{Kind: EventDone, Iter: iter, Payload: resp.Message.Content})
			return resp.Message.Content, nil
		}

		// 有工具调用：顺序执行，每个结果立即追加成 role=tool 消息。
		for _, call := range resp.ToolCalls {
			a.log("tool.exec", "name", call.Name, "id", call.ID)
			a.sink.Emit(Event{
				Kind:       EventToolCall,
				Iter:       iter,
				ToolName:   call.Name,
				ToolCallID: call.ID,
				Args:       call.Arguments,
			})
			output := a.executeTool(ctx, call)
			a.conv.Append(Message{
				Role:       RoleTool,
				ToolCallID: call.ID,
				Content:    output,
			})
			a.sink.Emit(Event{
				Kind:       EventToolResult,
				Iter:       iter,
				ToolName:   call.Name,
				ToolCallID: call.ID,
				Payload:    output,
			})
		}
	}

	a.sink.Emit(Event{Kind: EventError, Payload: ErrMaxIterations.Error()})
	a.sink.Emit(Event{Kind: EventDone, Payload: ErrMaxIterations.Error()})
	return "", ErrMaxIterations
}

// executeTool 执行单个工具调用，并把异常转换为对模型友好的文本反馈。
//
// 设计要点：这里不返回 error，而是始终返回"可读字符串"作为 tool_result 内容。
// 这样即使工具缺失或执行失败，模型也能在下一轮看到错误描述并尝试修复，
// 从而维持 one_loop.md "Harness 不插手决策" 的精神。
func (a *Agent) executeTool(ctx context.Context, call ToolCall) string {
	tool, ok := a.registry.Get(call.Name)
	if !ok {
		return fmt.Sprintf("工具错误: 未注册的工具 %q", call.Name)
	}

	// 避免把 null 传给工具；对于没有参数的工具，至少给一个空对象。
	args := call.Arguments
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage(`{}`)
	}

	out, err := tool.Execute(ctx, args)
	if err != nil {
		return fmt.Sprintf("工具执行异常: %v", err)
	}
	return out
}
