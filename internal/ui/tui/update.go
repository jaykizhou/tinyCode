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
//   - 鼠标：滚轮交给 viewport，左键点击用于切换气泡折叠。
//   - agent 事件：把结构化事件翻译成气泡。
//   - agent 完成：收尾 busy 状态；
//   - 其他：下放到子组件（viewport / textarea / spinner）。
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg), nil

	case tea.KeyMsg:
		return m.onKey(msg)

	case tea.MouseMsg:
		return m.onMouse(msg)

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
//
// viewport 高度采用“内容自适应 + 可用高度上限”的策略：
//   - 先按 5 段式扣除状态栏/输入头/输入框/hint/安全间距，算出可用最大高度 maxVpHeight；
//   - 再由 refreshViewport 按实际内容行数向下收缩，避免欢迎页等短内容下方出现大片空白、
//     把输入框挤到终端底部的观感问题。
func (m Model) onResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height

	// 与 newModel 保持一致：输入区默认 1 行，避免 placeholder 被折行重绘。
	inputHeight := 1
	m.viewport.Width = msg.Width
	m.input.SetWidth(msg.Width)
	m.input.SetHeight(inputHeight)
	m.refreshViewport()
	return m
}

// maxViewportHeight 返回当前窗口下 viewport 允许的最大高度。
// 状态栏 1 行 + 输入头 1 行 + 输入框 1 行 + hint 1 行 + 1 行安全间距。
func (m Model) maxViewportHeight() int {
	const reserved = 1 + 1 + 1 + 1 + 1
	vpHeight := m.height - reserved
	if vpHeight < 3 {
		vpHeight = 3
	}
	return vpHeight
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
	// 一轮对话结束：将本轮产生的气泡批量折叠，只保留前两行预览，
	// 减少历史轮次对视线的干扰。user 气泡通常也很短，isCollapsible 会自动将其忽略。
	m.collapseTurn(m.turnStartIdx)
	m.refreshViewport()
	return m
}

// collapseTurn 把从 from 下标开始到 history 末尾的所有气泡置为折叠态。
func (m *Model) collapseTurn(from int) {
	if from < 0 {
		from = 0
	}
	for i := from; i < len(m.history); i++ {
		m.history[i].collapsed = true
	}
}

// onMouse 处理鼠标事件。
//
// 设计原则：
//   - 默认将所有鼠标事件通过 updateChildren 转发给子组件，
//     这样 viewport 的滚轮滚动和 textarea 的光标定位等行为都能正常工作；
//   - 仅对 “左键按下 + Y 落在气泡头部行” 这个精准情境切换折叠状态，
//     避免用户拖选正文、或者鼠标摆动时的 release/motion 事件意外触发折叠。
//
// 重要：折叠触发之后仍然返回 m,nil 不转发，给用户一个明确的交互闭环，
// 防止视口同一点击事件被 textarea 二次解释导致 cursor 跳动。
func (m Model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		const statusHeight = 1
		if msg.Y >= statusHeight && msg.Y < statusHeight+m.viewport.Height {
			contentLine := msg.Y - statusHeight + m.viewport.YOffset
			for _, r := range m.bubbleRanges {
				// 仅当点击落在气泡“头行”（range.start）时才触发折叠切换。
				// 若放宽到整个 range 区间，用户在气泡正文上的每一次点击（
				// 包括文本拖选起点、多次点击等）都会被当做折叠/展开来进行，
				// 折展变化会导致气泡在视口内上下跃迁、后续点击命中不同气泡，
				// 视觉上恰好形成“标题重复出现多次”的故障现象。
				if contentLine == r.start {
					if r.idx >= 0 && r.idx < len(m.history) {
						m.history[r.idx].collapsed = !m.history[r.idx].collapsed
						m.refreshViewport()
					}
					return m, nil
				}
			}
		}
	}
	// 其余情形（滚轮、按键释放、移动、点击空白区或非头行行）都下放给子组件：
	// 这维持 viewport 滚轮滚动与 textarea 光标交互的完整行为，
	// 也比“提前过滤 IsWheel”更鲁棒——在 Windows Terminal / PowerShell 下，部分终端
	// 会将滚轮事件的 Button 报外为 None 或新类型，IsWheel() 返回 false 会导致错过
	// 滚动；全量下发 viewport 后由 viewport.Update 自行根据 MouseWheelEnabled 筛选。
	return m.updateChildren(msg)
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
// 同时将每个气泡的行号范围保存到 m.bubbleRanges，供鼠标点击定位使用。
//
// 高度自适应：当内容行数少于可用最大高度时（如欢迎页），viewport 按内容行数收缩，
// 避免展示区下方出现大片空白、输入框被挤到终端底部的观感问题。
func (m *Model) refreshViewport() {
	content, ranges := m.renderHistory()
	m.bubbleRanges = ranges

	maxH := m.maxViewportHeight()
	lines := strings.Count(content, "\n") + 1
	if lines < maxH {
		m.viewport.Height = lines
	} else {
		m.viewport.Height = maxH
	}

	m.viewport.SetContent(content)
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
