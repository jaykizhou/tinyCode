package bootstrap

// Package bootstrap 负责按 RuntimeConfig 装配 Agent 及其依赖。
//
// 把"如何组装对象"集中在这里，可以让 UI 层（TUI / REPL）共用一份装配逻辑，
// 也让未来替换模型 provider 或新增工具时只改动一处。

import (
	"fmt"

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
//   - RuntimeConfig.SystemPrompt 非空时覆盖内置提示。
//
// 扩展点：未来接入新的 Provider 只需在这里按 cfg 选择不同的 Model 实现。
func Build(cfg config.RuntimeConfig, opts Options) (*agent.Agent, error) {
	client := openai.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

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
		return nil, fmt.Errorf("装配 Agent 失败: %w", err)
	}
	return a, nil
}
