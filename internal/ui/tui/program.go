// Package tui 实现基于 Bubble Tea 的交互式终端 UI。
//
// 设计要点：
//   - 消费 agent.EventSink 的结构化事件，不与具体模型/工具耦合；
//   - MVU 拆分：model / update / view / runner / events / styles / keys；
//   - 单面板聊天式：顶部状态栏 + 中部对话区（viewport） + 底部输入框（textarea）。
package tui

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"tinycode/internal/agent"
	"tinycode/internal/cli/bootstrap"
	"tinycode/internal/cli/config"
)

// Run 启动 TUI，阻塞直到用户退出或 ctx 取消。
//
// 注意：TUI 占用 alt-screen 后，Cobra 默认的 signal 处理可能与 Bubble Tea 冲突，
// 因此这里用一个派生的 NotifyContext 只监听 SIGTERM（Ctrl+C 由 TUI 键位接管）。
func Run(parent context.Context, cfg config.RuntimeConfig) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGTERM)
	defer stop()

	sink := newChannelSink(64)
	a, art, err := bootstrap.Build(cfg, bootstrap.Options{
		ExtraAgentOptions: []agent.Option{
			agent.WithEventSink(sink),
		},
	})
	if err != nil {
		return err
	}
	// 开启观测时，进程退出前关闭 JSONL 文件句柄。
	if art.TraceCloser != nil {
		defer art.TraceCloser.Close()
	}

	m := newModel(ctx, a, cfg, sink)
	m.tracePath = art.TracePath

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
	)

	// 程序退出后再关闭 sink，保证 RunLoop 即使已结束也不会写入已关闭 channel。
	defer sink.Close()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI 运行失败: %w", err)
	}
	return nil
}
