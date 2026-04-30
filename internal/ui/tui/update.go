package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"tinycode/internal/agent"
	"tinycode/internal/cli/config"
)

// copyFeedbackMsg 是复制反馈倒计时的 tick 消息。
type copyFeedbackMsg struct{}

// Update 是 MVU 的核心：把输入的 tea.Msg 路由到具体处理函数。
//
// 分派规则：
//   - 窗口尺寸：resize。
//   - 按键：根据 keyMap 分流。
//   - agent 事件：把结构化事件翻译成气泡。
//   - agent 完成：收尾 busy 状态；
//   - 其他：下放到子组件（viewport / textarea / spinner）。
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg), nil

	case tea.KeyMsg:
		return m.onKey(msg)

	case agentEventMsg:
		return m.onAgentEvent(agent.Event(msg))

	case agentDoneMsg:
		return m.onAgentDone(msg), nil

	case eventsClosedMsg:
		// 事件通道关闭意味着 sink 被关了（TUI 退出流程），不再继续等待。
		return m, nil

	case copyFeedbackMsg:
		// 每次 tick 递减计数，归零后停止调度。
		if m.copyFeedback > 0 {
			m.copyFeedback--
			if m.copyFeedback > 0 {
				return m, scheduleCopyFeedbackTick()
			}
		}
		return m, nil
	}

	return m.updateChildren(msg)
}

// onResize 处理终端尺寸变化。按高度比例分配 viewport / textarea。
func (m Model) onResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height

	statusHeight := 1
	inputHeaderHeight := 1
	inputHeight := 3
	hintHeight := 1
	// 1 行安全间距（防止终端满屏后 textarea 在最后一行被部分遮挡）。
	vpHeight := msg.Height - statusHeight - inputHeaderHeight - inputHeight - hintHeight - 1
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Width = msg.Width
	m.viewport.Height = vpHeight
	m.input.SetWidth(msg.Width)
	m.input.SetHeight(inputHeight)
	m.refreshViewport()
	return m
}

// onKey 处理按键。
//
// Ctrl+C 在 busy 时取消 RunLoop；非 busy 时退出程序（Ctrl+D 也可退出）。
// Ctrl+Y 复制当前对话纯文本到系统剪贴板。
// Enter 提交，Shift+Enter 交由 textarea 自己处理换行。
func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		if m.cancelRun() {
			m.appendBubble(bubble{
				kind:    bubbleInfo,
				header:  "系统",
				content: "已请求取消当前对话…",
			})
			m.refreshViewport()
			return m, nil
		}
		return m, tea.Quit

	case keyMatches(msg, m.keys.Quit):
		return m, tea.Quit

	case keyMatches(msg, m.keys.ClearHist):
		m.history = nil
		m.refreshViewport()
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		m.viewport.ScrollUp(5)
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		m.viewport.ScrollDown(5)
		return m, nil

	case keyMatches(msg, m.keys.Copy):
		return m.onCopy()

	case keyMatches(msg, m.keys.Submit):
		if cmd := m.submitInput(); cmd != nil {
			m.refreshViewport()
			return m, cmd
		}
		// 空输入时不拦截 Enter，让 textarea 自己处理（仍然会清空，不影响体验）。
	}

	return m.updateChildren(msg)
}

// onCopy 把当前 history 的纯文本（剥离 ANSI 转义码）写入系统剪贴板。
func (m Model) onCopy() (tea.Model, tea.Cmd) {
	plain := historyPlainText(m.history, m.cfg)
	if err := clipboard.WriteAll(plain); err != nil {
		// 写入失败时追加一条错误气泡，不中断程序。
		m.appendBubble(bubble{
			kind:    bubbleError,
			header:  "复制失败",
			content: err.Error(),
		})
		m.refreshViewport()
		return m, nil
	}
	// 启动反馈倒计时（约 2 秒 = 4 tick × 500ms）。
	m.copyFeedback = 4
	return m, scheduleCopyFeedbackTick()
}

// scheduleCopyFeedbackTick 返回一个 500ms 后触发 copyFeedbackMsg 的 Cmd。
func scheduleCopyFeedbackTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		return copyFeedbackMsg{}
	})
}

// historyPlainText 把 history 转换为纯文本，剥离所有 ANSI 转义码。
// history 为空时返回欢迎文本的纯文本版本。
func historyPlainText(history []bubble, cfg config.RuntimeConfig) string {
	if len(history) == 0 {
		return ansi.Strip(welcomeText(cfg, "", 0))
	}
	var sb strings.Builder
	for i, b := range history {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		switch b.kind {
		case bubbleUser:
			sb.WriteString("▶ " + b.header + "\n")
			sb.WriteString(b.content)
		case bubbleAssistant:
			sb.WriteString("◆ " + b.header + "\n")
			sb.WriteString(b.content)
		case bubbleToolCall:
			sb.WriteString("▷ " + b.header + "\n")
			sb.WriteString(b.content)
		case bubbleToolResult:
			sb.WriteString("◀ " + b.header + "\n")
			sb.WriteString(b.content)
		case bubbleError:
			sb.WriteString("✖ " + b.header + "\n")
			sb.WriteString(b.content)
		case bubbleInfo:
			sb.WriteString("ℹ " + b.content)
		default:
			sb.WriteString(b.content)
		}
	}
	return sb.String()
}

// onAgentEvent 把结构化事件翻译为气泡；处理完后重新挂回事件监听 Cmd，形成"事件流水线"。
func (m Model) onAgentEvent(e agent.Event) (tea.Model, tea.Cmd) {
	switch e.Kind {
	case agent.EventIterStart:
		m.currIter = e.Iter

	case agent.EventAssistantReply:
		if strings.TrimSpace(e.Payload) != "" {
			m.appendBubble(bubble{
				kind:    bubbleAssistant,
				header:  fmt.Sprintf("Agent · 轮次 %d", e.Iter),
				content: e.Payload,
			})
		}

	case agent.EventToolCall:
		m.appendBubble(bubble{
			kind:     bubbleToolCall,
			header:   fmt.Sprintf("tool · %s", e.ToolName),
			content:  argsSummary(e.Args),
			metadata: e.ToolCallID,
		})

	case agent.EventToolResult:
		m.appendBubble(bubble{
			kind:     bubbleToolResult,
			header:   fmt.Sprintf("tool result · %s", e.ToolName),
			content:  e.Payload,
			metadata: e.ToolCallID,
		})

	case agent.EventError:
		m.appendBubble(bubble{
			kind:    bubbleError,
			header:  "错误",
			content: e.Payload,
		})

	case agent.EventDone:
		// 最终结果气泡交给 agentDoneMsg 兜底，这里不重复追加。
	}

	m.refreshViewport()
	return m, waitForEvent(m.sink.Events())
}

// onAgentDone 处理一次 RunLoop 结束。无论成功还是失败都会清除 busy 状态。
func (m Model) onAgentDone(msg agentDoneMsg) Model {
	m.busy = false
	if m.runCancel != nil {
		m.runCancel()
		m.runCancel = nil
	}
	m.applyInputMode(false)

	if msg.err != nil {
		// 取消导致的错误仅提示，不再额外追加（onKey 已追加过"已请求取消"）。
		if !strings.Contains(msg.err.Error(), "context canceled") {
			m.appendBubble(bubble{
				kind:    bubbleError,
				header:  "错误",
				content: msg.err.Error(),
			})
		}
	}
	m.refreshViewport()
	return m
}

// updateChildren 把当前未分派的消息交给子组件更新。
// 注意：一条 tea.Msg 可以同时被多个子组件感兴趣（如 WindowSizeMsg），
// 但本项目中我们在 onResize 显式接管了窗口消息，所以这里只需按序转发。
func (m Model) updateChildren(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.busy {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	var tcmd tea.Cmd
	m.input, tcmd = m.input.Update(msg)
	cmds = append(cmds, tcmd)

	var vcmd tea.Cmd
	m.viewport, vcmd = m.viewport.Update(msg)
	cmds = append(cmds, vcmd)

	return m, tea.Batch(cmds...)
}

// appendBubble 追加气泡。注意：调用方还需 refreshViewport 来同步内容。
func (m *Model) appendBubble(b bubble) {
	m.history = append(m.history, b)
}

// refreshViewport 根据 history 重渲染 viewport 内容，并自动滚到底部。
func (m *Model) refreshViewport() {
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
}

// argsSummary 把工具调用入参 JSON 压缩成一行显示，方便阅读。
func argsSummary(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	// 尝试紧凑打印，以免多行 JSON 撑爆气泡
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

// keyMatches 是 key.Binding.Matches 的一层薄封装，避免散落在 switch 里。
func keyMatches(msg tea.KeyMsg, b key.Binding) bool {
	return key.Matches(msg, b)
}
