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
		tea.WithContext(ctx),
		// 开启鼠标事件：
		//   1. viewport 的 MouseWheelEnabled 默认开启，但仅当 Program
		//      放行 MouseMsg 时才能收到滚轮，否则终端滚轮在 AltScreen 里完全失效；
		//   2. 气泡点击折叠/展开交互也依赖左键 MouseMsg。
		//   使用 CellMotion 而非 AllMotion：只在按键/滚动时上报，避免移动洪流浪费事件。
		tea.WithMouseCellMotion(),
	)

	// 程序退出后再关闭 sink，保证 RunLoop 即使已结束也不会写入已关闭 channel。
	defer sink.Close()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI 运行失败: %w", err)
	}
	return nil
}
