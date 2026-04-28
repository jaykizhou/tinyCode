package agent

import "context"

// Model 定义了 Agent 与大模型之间的最小契约。
//
// 任何支持"多轮对话 + 工具调用"的大模型，都可以通过实现这个接口接入 Agent。
// 设计上只要求同步一次调用（Complete），流式接入留作后续扩展点。
type Model interface {
	// Name 返回模型名称，便于日志与调试。
	Name() string

	// Complete 发送一次补全请求并返回响应。
	// 实现需要自行处理 API 协议细节（例如 OpenAI 的 messages 格式、tools 字段等）。
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
