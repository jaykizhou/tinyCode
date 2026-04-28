package agent

import "errors"

// 本文件集中定义 Agent 可能返回的可识别错误，便于上层使用 errors.Is 做判断。
var (
	// ErrModelRequired 表示构造 Agent 时没有通过 WithModel 注入模型。
	ErrModelRequired = errors.New("agent: model provider is required")

	// ErrMaxIterations 表示 Agent 主循环超过最大迭代次数仍未产出最终回复，
	// 通常意味着模型陷入了"不断调用工具"的死循环，需要上层介入。
	ErrMaxIterations = errors.New("agent: max iterations exceeded")

	// ErrToolNotFound 表示模型请求调用的工具没有在 Registry 中注册。
	// 该错误会被包装成 tool_result 返回给模型，不会中断主循环。
	ErrToolNotFound = errors.New("agent: tool not found")
)
