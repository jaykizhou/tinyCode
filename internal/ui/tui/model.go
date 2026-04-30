package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"tinycode/internal/agent"
	"tinycode/internal/cli/config"
)

// Model 是 Bubble Tea 的 MVU Model。
//
// 设计取舍：
//   - 不在 Model 里直接持有 context.CancelFunc，改为按轮次创建 runCtx/runCancel，
//     这样 Ctrl+C 在 busy 状态下只取消"当前 RunLoop"，不会影响程序整体生命周期；
//   - history 是整个会话的 UI 投影，来源于事件流（可增不可改）；
//   - busy 标志用于禁用连续提交、决定是否展示 spinner。
type Model struct {
	// 外部依赖
	agent *agent.Agent
	cfg   config.RuntimeConfig
	ctx   context.Context
	sink  *channelSink

	// 子组件
	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model

	// 状态
	keys      keyMap
	history   []bubble
	busy      bool
	currIter  int
	runCancel context.CancelFunc // 仅在 busy=true 时非 nil
	width     int
	height    int
	tracePath string // 当前运行的观测日志路径；为空即未开启
}

// newModel 在 program.go 中被调用，封装初始默认值，便于单元测试替换。
func newModel(ctx context.Context, a *agent.Agent, cfg config.RuntimeConfig, sink *channelSink) Model {
	ta := textarea.New()
	ta.Placeholder = "输入消息，Enter 发送，Shift+Enter 换行，Ctrl+D 退出..."
	ta.Prompt = "▎ "
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.Focus()

	vp := viewport.New(80, 20)
	vp.SetContent(welcomeText(cfg))

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		agent:    a,
		cfg:      cfg,
		ctx:      ctx,
		sink:     sink,
		viewport: vp,
		input:    ta,
		spinner:  sp,
		keys:     defaultKeyMap(),
	}
}

// Init 实现 tea.Model：启动监听事件通道 + 启动 spinner tick。
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		waitForEvent(m.sink.Events()),
	)
}

// welcomeText 渲染初始 viewport 内容。
func welcomeText(cfg config.RuntimeConfig) string {
	return styles.hintText.Render(
		"欢迎使用 TinyCode TUI。\n"+
			"模型: "+cfg.Model+"  |  工作目录: "+cfg.WorkDir+"\n"+
			"Enter 发送 · Shift+Enter 换行 · Ctrl+L 清屏 · Ctrl+D 退出 · Ctrl+C 取消当前对话",
	) + "\n"
}
