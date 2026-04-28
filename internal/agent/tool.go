package agent

import (
	"context"
	"encoding/json"
	"sync"
)

// Tool 是所有可被 Agent 调度的工具必须实现的接口。
//
// 该接口故意设计得非常小，理由与 one_loop.md 的精神一致：
// "Agency 来自模型的训练，不是来自编排代码"——工具只需忠实执行，
// 不要在接口层面塞入任何业务控制流。
type Tool interface {
	// Name 是工具唯一标识，对应 OpenAI tools[].function.name。
	Name() string

	// Description 提供给模型理解"什么时候调用它"。写得越准确，模型决策越好。
	Description() string

	// Parameters 返回参数的 JSON Schema，作为 tools[].function.parameters 发给模型。
	Parameters() json.RawMessage

	// Execute 执行工具并返回纯文本结果。
	//
	// 约定：当"业务上失败"（例如命令执行返回非 0 退出码）时，
	// 建议将错误信息拼入返回文本并返回 nil error，这样模型可以看到错误并自行修复；
	// 只有当"工具自身出问题"（参数解析失败、内部异常）时才返回 error。
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// Registry 是一个简单的工具注册表（dispatch map）。
//
// 它既负责给 Agent 提供运行时的 Tool 查找，也负责生成发送给模型的 ToolDefinition 列表。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string // 保留注册顺序，生成 Definitions 时稳定输出
}

// NewRegistry 构造一个空注册表。
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register 注册一个工具。若同名工具已存在则覆盖（便于测试替身场景）。
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := t.Name()
	if _, ok := r.tools[name]; !ok {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

// Get 按名称查找工具。
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Definitions 返回发送给模型的工具清单（按注册顺序）。
func (r *Registry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}
