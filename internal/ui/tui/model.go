package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tinycode/internal/agent"
	"tinycode/internal/cli/config"
)

// Model 是 Bubble Tea 的 MVU Model。
//
// 设计取舍：
//   - 不在 Model 里直接持有 context.CancelFunc，改为按轮次创建 runCtx/runCancel。
//     这样 Ctrl+C 在 busy 状态下只取消"当前 RunLoop"，不会影响程序整体生命周期；
//   - history 是整个会话的 UI 投影，来源于事件流（可增不可改）。
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

	// 复制反馈：显示"已复制"提示的剩余帧数（>0 时在状态栏展示）
	copyFeedback int

	// turnStartIdx 记录当前/最近一轮对话在 history 中的起始下标。
	// submitInput 时更新为 user 气泡的下标，onAgentDone 据此批量折叠本轮气泡。
	turnStartIdx int

	// bubbleRanges 由 refreshViewport 在每次重渲染时填入，
	// 记录每个气泡在 viewport 内容里的行号范围，供鼠标点击反查使用。
	bubbleRanges []bubbleRange
}

// newModel 在 program.go 中被调用，封装初始默认值，便于单元测试替换。
func newModel(ctx context.Context, a *agent.Agent, cfg config.RuntimeConfig, sink *channelSink) Model {
	ta := textarea.New()
	// Placeholder 和 Prompt 来自 applyInputMode 动态设置（按 busy 切换），
	// 在构造末尾一次性初始化为 idle 态。
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	// 初始单行高度：bubbles 的 textarea 在 Windows 终端下对中文 placeholder
	// 的可视宽度计算偏小，一旦 height>1 就会在第二行把 placeholder 折行重绘，
	// 形成“输入消息…”重复出现的错觉。按 1 行起步，Shift+Enter 换行时
	// textarea 会自己按内容增高，不损失可用性。
	ta.SetHeight(1)
	ta.Focus()

	// viewport 初始内容由 renderHistory 统一管理，不在此预设，
	// 避免与 refreshViewport 首次调用产生重复渲染。
	vp := viewport.New(80, 20)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := Model{
		agent:    a,
		cfg:      cfg,
		ctx:      ctx,
		sink:     sink,
		viewport: vp,
		input:    ta,
		spinner:  sp,
		keys:     defaultKeyMap(),
	}
	m.applyInputMode(false)
	return m
}

// applyInputMode 根据 busy 状态同步 textarea 的 Prompt。
//
// 为什么不用 m.input.Prompt + SetWidth：
//
//	bubbles 的 SetWidth 会用 uniseg.StringWidth(m.Prompt) 估算 promptWidth，
//	而我们的 Prompt 带 ANSI 颜色转义（styles.inputPromptXxx.Render(...)），
//	uniseg 不认这些控制序列，会把转义字节当作普通字符累加宽度，得到一个
//	远大于视觉宽度（2）的值；随后 m.width（每行可容纳的内容宽度）被压缩到
//	接近 0，textarea 会对任意短输入做硬换行，加上每行自带 prompt 就出现
//	“‼” 下面再重复多行 “❯” 的伪多行现象。
//
// 解决方案：改用 SetPromptFunc(promptWidth=2, ...) 显式告诉 bubbles 这个
// prompt 的视觉宽度就是 2 列——之后 SetWidth 的 uniseg 路径会被短路，
// 不再重新计算 promptWidth，m.width 始终正确。prompt 字符串本身仍可以
// 携带 ANSI 颜色，终端按正常方式渲染。
//
// Placeholder 统一置空：相同原因——uniseg 同样会把 placeholder 按畸变
// 宽度折行；视觉提示已由输入头胶囊标签（"▸ 你的输入" / "▸ 正在处理"）和
// 底部 hint 行承担，placeholder 是冗余，干脆不设。
func (m *Model) applyInputMode(busy bool) {
	var prompt string
	if busy {
		prompt = styles.inputPromptBusy.Render("‼") + " "
	} else {
		prompt = styles.inputPromptIdle.Render("❯") + " "
	}
	// promptWidth=2 对应 "❯ "/"‼ " 的可视宽度（1 个符号 + 1 个空格）。
	m.input.SetPromptFunc(2, func(int) string { return prompt })
	m.input.Placeholder = ""
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
//
// 采用双栏圆角卡片的版式：左栏是品牌与运行上下文、右栏是快速入门与观测状态。
// 当 width 不足以容纳双栏时（如初始未收到 WindowSizeMsg），降级为单行提示，
// 避免超宽折行打破语义。
func welcomeText(cfg config.RuntimeConfig, tracePath string, width int) string {
	const minFramed = 72

	target := width
	if target <= 0 {
		target = 100
	}
	if target > 110 {
		target = 110
	}
	if target < minFramed {
		return legacyWelcomeText(cfg, tracePath)
	}

	// 外框占 2 个横向单位；中间竖线 1；左右内容各占 1 格 padding。
	innerW := target - 2
	leftInnerW := innerW/2 - 1
	rightInnerW := innerW - leftInnerW - 1

	leftLines := buildWelcomeLeft(cfg, leftInnerW)
	rightLines := buildWelcomeRight(tracePath, rightInnerW)

	// 对齐行数：较短的一栏在尾部补空行。
	for len(leftLines) < len(rightLines) {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < len(leftLines) {
		rightLines = append(rightLines, "")
	}

	title := styles.welcomeTitle.Render("tinyCode") +
		styles.welcomeDim.Render(" · A Tiny Coding Agent")
	return framePanel(title, leftLines, rightLines, leftInnerW, rightInnerW) + "\n"
}

// legacyWelcomeText 是窄终端降级时的纯文本版欢迎提示。
func legacyWelcomeText(cfg config.RuntimeConfig, tracePath string) string {
	var sb strings.Builder
	sb.WriteString(styles.welcomeGreeting.Render("Welcome back!") + "\n")
	sb.WriteString(styles.welcomeDim.Render("tinyCode · A Tiny Coding Agent") + "\n")
	sb.WriteString(styles.welcomeField.Render("模型  ") + styles.welcomeValue.Render(cfg.Model) + "\n")
	sb.WriteString(styles.welcomeField.Render("目录  ") + styles.welcomeValue.Render(shortPath(cfg.WorkDir)) + "\n")
	if tracePath != "" {
		sb.WriteString(styles.welcomeField.Render("观测  ") + styles.welcomeValue.Render(tracePath) + "\n")
	}
	sb.WriteString(styles.hintText.Render("Enter 发送 · Shift+Enter 换行 · Ctrl+L 清屏 · Ctrl+D 退出 · Ctrl+Y 复制") + "\n")
	return sb.String()
}

// buildWelcomeLeft 构造左栏文本（品牌问候 + Logo + 当前上下文）。
func buildWelcomeLeft(cfg config.RuntimeConfig, innerW int) []string {
	logo := []string{
		"   ◢◤  ◢◤◤◣   ",
		"  ▐▌▉█▌ ▐█▄◣    ",
		"   ▀▌   ▀▌▀    ",
	}

	lines := []string{
		"",
		center(styles.welcomeGreeting.Render("Welcome back!"), innerW),
		"",
	}
	for _, l := range logo {
		lines = append(lines, center(styles.welcomeLogo.Render(l), innerW))
	}
	lines = append(lines, "")

	model := cfg.Model
	if model == "" {
		model = "(default)"
	}
	work := cfg.WorkDir
	if work == "" {
		work = "."
	}

	// 模型 · API 信息
	lines = append(lines, field("模型", model, innerW))
	lines = append(lines, field("目录", shortenForField(work, innerW-6), innerW))
	lines = append(lines, "")
	return lines
}

// buildWelcomeRight 构造右栏文本（快速开始 + 观测状态）。
func buildWelcomeRight(tracePath string, innerW int) []string {
	lines := []string{
		"",
		styles.welcomeLabel.Render("快速开始"),
		kvLine("Enter", "发送消息"),
		kvLine("Shift+Enter", "换行"),
		kvLine("Ctrl+L", "清屏历史"),
		kvLine("Ctrl+Y", "复制当前会话"),
		kvLine("Ctrl+C", "取消当前运行"),
		kvLine("Ctrl+D", "退出程序"),
		"",
		styles.welcomeRule.Render(strings.Repeat("─", innerW)),
		"",
		styles.welcomeLabel.Render("观测日志"),
	}
	if tracePath != "" {
		lines = append(lines,
			styles.welcomeBullet.Render("● ")+styles.welcomeDesc.Render("已开启")+styles.welcomeDim.Render(" (jsonl)"),
			styles.welcomeDim.Render(shortenForField(tracePath, innerW-2)),
		)
	} else {
		lines = append(lines,
			styles.welcomeDim.Render("○ 未开启"),
			styles.welcomeDim.Render("  --trace 或 TINYCODE_TRACE=1 启用"),
		)
	}
	lines = append(lines, "")
	return lines
}

// framePanel 把左右两栏拼装成带标题的圆角卡片。
//
// 手工构造边框而非依赖 lipgloss.Border，原因：需要在顶边中间嵌入标题，
// 而 lipgloss 自带的 Border 实现不支持该一类能力。
func framePanel(title string, left, right []string, leftW, rightW int) string {
	const (
		tl = "╭"
		tr = "╮"
		bl = "╰"
		br = "╯"
		hz = "─"
		vt = "│"
	)
	b := styles.welcomeBorder
	totalInner := leftW + 1 + rightW // 左内容 + 中竖线 + 右内容

	// 顶边：╭─── title ───╮
	titleText := " " + title + " "
	leftDashes := 3
	remaining := totalInner - leftDashes - lipgloss.Width(titleText)
	if remaining < 1 {
		remaining = 1
	}
	top := b.Render(tl+strings.Repeat(hz, leftDashes)) + titleText +
		b.Render(strings.Repeat(hz, remaining)+tr)

	bot := b.Render(bl + strings.Repeat(hz, totalInner) + br)

	vert := b.Render(vt)
	var body []string
	for i := 0; i < len(left); i++ {
		lp := padRightVisual(left[i], leftW)
		rp := padRightVisual(right[i], rightW)
		body = append(body, vert+lp+vert+rp+vert)
	}

	return top + "\n" + strings.Join(body, "\n") + "\n" + bot
}

// padRightVisual 按可视宽度将文本右侧补空至 width 列；超长则截断。
func padRightVisual(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	// 缩进场景很少超宽，简单用 ansi.Truncate 的替代：直接以原串返回。
	return s
}

// center 把文本在 width 宽度内水平居中（按可视宽度）。
func center(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s
}

// field 渲染一行字段："  字段  值"，用于左栏的模型/目录等展示。
func field(name, value string, innerW int) string {
	label := styles.welcomeField.Render(name)
	val := styles.welcomeValue.Render(value)
	return "  " + label + "  " + val
}

// kvLine 渲染右栏的"按键 · 描述"行。
func kvLine(k, v string) string {
	return "  " + styles.welcomeKey.Render(padKey(k, 11)) + "  " +
		styles.welcomeDesc.Render(v)
}

// padKey 把按键名按可视宽度右侧补空至 width，保证右栏列对齐。
func padKey(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// shortenForField 当字段值过长时，保留尾部并用。。。前缀替代，控制到 max 宽。
func shortenForField(s string, max int) string {
	if max <= 3 {
		return s
	}
	w := lipgloss.Width(s)
	if w <= max {
		return s
	}
	runes := []rune(s)
	// 近似处理：以 rune 长度粗估，终端运行的路径绝大多数为 ASCII。
	tail := max - 3
	if tail > len(runes) {
		tail = len(runes)
	}
	return "..." + string(runes[len(runes)-tail:])
}
