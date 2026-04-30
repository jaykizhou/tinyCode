package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"tinycode/internal/agent"
)

// bubbleKind 区分气泡的视觉样式。
type bubbleKind int

const (
	bubbleUser bubbleKind = iota
	bubbleAssistant
	bubbleToolCall
	bubbleToolResult
	bubbleError
	bubbleInfo
)

// bubble 是 TUI 中渲染的一条对话单元。
//
// 之所以不直接复用 agent.Message，是因为 UI 需要追加"工具调用快照"、
// "错误气泡"等 Agent 侧不关心的语义。
type bubble struct {
	kind     bubbleKind
	header   string // 例如 "你"/"Agent"/"tool: shell"
	content  string // 正文
	metadata string // 辅助信息（如 tool_call id / args 摘要）

	// collapsed 标记当前气泡是否处于折叠展示状态。
	// 一轮对话结束时由 onAgentDone 批量置 true，鼠标左键点击时翻转。
	collapsed bool
}

// bubbleRange 记录一个气泡在 viewport 内容里的行号范围（左闭右开），
// 用于把鼠标点击的屏幕坐标反查到具体气泡，触发折叠/展开。
type bubbleRange struct {
	idx   int // 对应 Model.history 下标
	start int // 内容首行（inclusive）
	end   int // 内容末行（exclusive）
}

// agentEventMsg 在 Update 中把 agent.Event 投递为 tea.Msg。
type agentEventMsg agent.Event

// agentDoneMsg 表示一次 RunLoop 的最终结果（回复或错误）。
type agentDoneMsg struct {
	reply string
	err   error
}

// eventsClosedMsg 表示事件通道已关闭，UI 停止继续监听。
type eventsClosedMsg struct{}

// waitForEvent 生成一个从事件通道读取的 tea.Cmd。
//
// 使用模式：每收到一条事件就重新调度一次，直到通道关闭。
// 这样即可让 Bubble Tea 的 Update 循环线性消费事件，
// 无需引入共享可变状态。
func waitForEvent(ch <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return eventsClosedMsg{}
		}
		return agentEventMsg(e)
	}
}

// channelSink 把 Agent 的结构化事件投递到带缓冲 channel，
// 保证 Emit 不会阻塞 Agent 主循环（慢消费者会被丢弃）。
type channelSink struct {
	ch chan agent.Event
}

func newChannelSink(buf int) *channelSink {
	if buf <= 0 {
		buf = 64
	}
	return &channelSink{ch: make(chan agent.Event, buf)}
}

// Emit 实现 agent.EventSink。
//
// 采用非阻塞 select：若 UI 暂时没来得及消费，事件会被丢弃而不是拖死 RunLoop。
// 对 coding agent 来说，偶尔丢一条事件可接受——权威状态仍在 Conversation 中。
func (s *channelSink) Emit(e agent.Event) {
	select {
	case s.ch <- e:
	default:
	}
}

// Events 返回只读事件通道，供 waitForEvent 使用。
func (s *channelSink) Events() <-chan agent.Event { return s.ch }

// Close 关闭事件通道，必须在 RunLoop 彻底结束后调用。
func (s *channelSink) Close() { close(s.ch) }
