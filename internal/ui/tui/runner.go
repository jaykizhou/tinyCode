package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// runAgentCmd 启动一次 Agent.RunLoop。
//
// 关键点：
//   - 在独立 goroutine 中运行 RunLoop，通过 tea.Cmd 返回最终 agentDoneMsg；
//   - 使用 WithCancel 派生一个"本轮可取消"的子 context，挂在 Model 上，
//     Ctrl+C 时由 Update 调用 cancel，优雅中断长工具或模型请求。
func (m *Model) runAgentCmd(input string) tea.Cmd {
	runCtx, cancel := context.WithCancel(m.ctx)
	m.runCancel = cancel
	m.busy = true

	return func() tea.Msg {
		reply, err := m.agent.RunLoop(runCtx, input)
		return agentDoneMsg{reply: reply, err: err}
	}
}

// submitInput 把 textarea 中的文本提交给 Agent。
//
// 返回值：nil 表示没有可提交内容（空白或 busy），否则返回驱动 RunLoop 的命令。
// 该方法会同时把用户气泡追加到 history，保证 UI 立刻给出反馈。
func (m *Model) submitInput() tea.Cmd {
	if m.busy {
		return nil
	}
	raw := m.input.Value()
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}

	m.appendBubble(bubble{
		kind:    bubbleUser,
		header:  "你",
		content: text,
	})
	// 记录本轮对话的起始下标（user 气泡自身），onAgentDone 据此批量折叠本轮产生的气泡。
	m.turnStartIdx = len(m.history) - 1
	m.input.Reset()
	m.applyInputMode(true)
	return tea.Batch(m.runAgentCmd(text), m.spinner.Tick)
}

// cancelRun 在 busy 状态下取消当前 RunLoop；非 busy 时返回 false，由调用方决定是否退出程序。
func (m *Model) cancelRun() bool {
	if !m.busy || m.runCancel == nil {
		return false
	}
	m.runCancel()
	return true
}
