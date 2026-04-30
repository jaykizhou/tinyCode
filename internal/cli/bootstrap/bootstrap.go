package bootstrap

// Package bootstrap 负责按 RuntimeConfig 装配 Agent 及其依赖。
//
// 把"如何组装对象"集中在这里，可以让 UI 层（TUI / REPL）共用一份装配逻辑，
// 也让未来替换模型 provider 或新增工具时只改动一处。

import (
	"fmt"
	"io"

	"tinycode/internal/agent"
	"tinycode/internal/cli/config"
	"tinycode/internal/model/openai"
	"tinycode/internal/tools/shell"
)

// Options 是 Build 的可扩展参数，便于 UI 层注入额外的 Agent Option
// （例如 TUI 注入 EventSink、REPL 注入 Logger）。
type Options struct {
	// ExtraAgentOptions 会在通用选项之后追加，可用于注入 UI 特定钩子。
	ExtraAgentOptions []agent.Option
}

// Build 根据 RuntimeConfig + Options 构造 *agent.Agent。
//
// 当前实现：
//   - 模型走 OpenAI 兼容客户端；
//   - 默认注册 shell 工具；
//   - RuntimeConfig.SystemPrompt 非空时覆盖内置提示；
//   - 开启 Trace 时注入 JSONL 文件观测器，路径由返回的 Artifacts.TracePath 送出。
//
// 扩展点：未来接入新的 Provider 只需在这里按 cfg 选择不同的 Model 实现。
func Build(cfg config.RuntimeConfig, opts Options) (*agent.Agent, Artifacts, error) {
	var art Artifacts

	clientOpts := []openai.Option{}
	if cfg.Trace {
		obs, err := openai.NewJSONLFileObserver(cfg.TraceDir)
		if err != nil {
			return nil, art, fmt.Errorf("初始化观测器失败: %w", err)
		}
		clientOpts = append(clientOpts, openai.WithObserver(obs))
		art.TracePath = obs.Path()
		art.TraceCloser = obs
	}
	client := openai.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model, clientOpts...)

	agentOpts := []agent.Option{
		agent.WithModel(client),
		agent.WithTools(shell.New(cfg.WorkDir)),
		agent.WithMaxIterations(cfg.MaxIterations),
	}
	if cfg.SystemPrompt != "" {
		agentOpts = append(agentOpts, agent.WithSystemPrompt(cfg.SystemPrompt))
	}
	agentOpts = append(agentOpts, opts.ExtraAgentOptions...)

	a, err := agent.NewAgent(agentOpts...)
	if err != nil {
		if art.TraceCloser != nil {
			_ = art.TraceCloser.Close()
		}
		return nil, Artifacts{}, fmt.Errorf("装配 Agent 失败: %w", err)
	}
	return a, art, nil
}

// Artifacts 是 Build 过程中产生的副产品（由 UI 层按需展示或收尾）。
//
// 设计动机：Trace 文件路径需要在 UI 启动时告知用户；同时由于文件句柄需要在进程退出前
// 关闭，UI 层可以在 defer 中调用 TraceCloser.Close() 以避免资源泄漏。
type Artifacts struct {
	TracePath   string    // 当前运行的 JSONL 日志绝对路径；未开启 Trace 时为空
	TraceCloser io.Closer // 对应的文件句柄，需在退出前 Close；未开启时为 nil
}
