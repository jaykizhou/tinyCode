package agent

// Option 使用 Functional Options 模式装配 Agent，
// 灵感来自 blades-main 的 AgentOption 设计：外部只需传 opts 列表，
// 内部结构字段可以自由演进而不破坏调用方。
type Option func(*Agent)

// WithModel 注入模型提供者，这是唯一的必选项。
func WithModel(m Model) Option {
	return func(a *Agent) { a.model = m }
}

// WithSystemPrompt 设置系统提示词。默认提示已经足够最小可用，调用方可用此项覆盖。
func WithSystemPrompt(s string) Option {
	return func(a *Agent) { a.systemPrompt = s }
}

// WithTools 追加一批工具到 Agent 的注册表。可以多次调用。
func WithTools(ts ...Tool) Option {
	return func(a *Agent) {
		for _, t := range ts {
			a.registry.Register(t)
		}
	}
}

// WithMaxIterations 设置主循环最大迭代次数，作为防止死循环的"安全阀"。
// 默认值 20 足以应对绝大多数多步工具调用场景。
func WithMaxIterations(n int) Option {
	return func(a *Agent) {
		if n > 0 {
			a.maxIterations = n
		}
	}
}

// WithLogger 注入一个可选的循环观察者（比如打印每一步工具调用）。
// 设计成函数而非接口，是为了让调用方用最轻的方式接入日志 / UI。
func WithLogger(fn func(event string, kv ...any)) Option {
	return func(a *Agent) { a.log = fn }
}

// WithEventSink 注入结构化事件订阅者，用于 TUI / 监控等需要感知主循环内部状态的场景。
// 与 WithLogger 并行存在：logger 产出人类可读日志，sink 产出结构化事件。
func WithEventSink(s EventSink) Option {
	return func(a *Agent) {
		if s != nil {
			a.sink = s
		}
	}
}
