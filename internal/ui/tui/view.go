package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View 实现 tea.Model：五段式布局 = 状态栏 + viewport + 输入头 + 输入框 + 底部快捷键。
//
// 设计说明：
//   - 所有具体样式集中在 styles 变量中，这里只做拼装。
//   - 输入头负责把“展示区”与“操作区”切开，避免用户混淆输入框和普通文本。
func (m Model) View() string {
	status := m.renderStatusBar()
	body := m.viewport.View()
	header := m.renderInputHeader()
	input := m.input.View()
	hint := styles.hintText.Render(m.renderHint())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		status,
		body,
		header,
		input,
		hint,
	)
}

// renderStatusBar 构造顶部单行状态栏。
func (m Model) renderStatusBar() string {
	var rightPart string
	switch {
	case m.copyFeedback > 0:
		// 复制成功反馈，优先级最高
		rightPart = styles.statusCopy.Render("✓ 已复制到剪贴板")
	case m.busy:
		rightPart = styles.statusBusy.Render(
			m.spinner.View() + " 思考中… (Ctrl+C 取消)",
		)
	default:
		rightPart = styles.statusVal.Render("就绪")
	}

	left := fmt.Sprintf(
		"%s %s  %s %s  %s %d",
		styles.statusKey.Render("model"),
		styles.statusVal.Render(m.cfg.Model),
		styles.statusKey.Render("cwd"),
		styles.statusVal.Render(shortPath(m.cfg.WorkDir)),
		styles.statusKey.Render("iter"),
		m.currIter,
	)

	width := m.width
	if width <= 0 {
		width = 80
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(rightPart) - 2
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + rightPart
	return styles.statusBar.Width(width).Render(bar)
}

// renderHint 在 viewport 与输入框之间给出一行操作提示。
func (m Model) renderHint() string {
	if m.busy {
		return "Ctrl+C 取消当前对话  ·  Ctrl+L 清屏  ·  Ctrl+Y 复制  ·  Ctrl+D 退出"
	}
	return "Enter 发送  ·  Shift+Enter 换行  ·  Ctrl+L 清屏  ·  Ctrl+Y 复制  ·  Ctrl+D 退出"
}

// renderInputHeader 渲染输入区顶部的分隔带。
//
// 它同时承担两个职责：
//  1. 视觉切割“展示区”与“操作区”，以胶囊形标签明指这里是输入区；
//  2. 在 busy 状态下切换为橙色主题并嵌入 spinner 与操作提示，
//     让用户一眼到底感知到 Agent 在处理中、不要等空或重复发送。
func (m Model) renderInputHeader() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	var (
		label  string
		right  string
		ruleFg lipgloss.Style
	)
	if m.busy {
		label = styles.inputLabelBusy.Render("▸ 正在处理")
		right = styles.statusBusy.Render(m.spinner.View() + " Agent 思考中… Ctrl+C 取消")
		ruleFg = styles.inputRuleBusy
	} else {
		label = styles.inputLabelIdle.Render("▸ 你的输入")
		right = styles.hintText.Render("Enter 发送")
		ruleFg = styles.inputRuleIdle
	}

	leftRule := ruleFg.Render("━━")
	pad := width - lipgloss.Width(leftRule) - lipgloss.Width(label) - lipgloss.Width(right) - 3
	if pad < 2 {
		pad = 2
	}
	midRule := ruleFg.Render(strings.Repeat("━", pad))
	return leftRule + " " + label + " " + midRule + " " + right
}

// renderHistory 把 history 中的气泡一一渲染成带样式的字符串。
//
// 风格：
//   - 不画边框（避免中英文等宽问题）；
//   - 每个气泡之间空一行，提升可读性；
//   - 工具输出类气泡做宽度自适应截断（超长内容保留头尾）。
func (m Model) renderHistory() string {
	if len(m.history) == 0 {
		return welcomeText(m.cfg, m.tracePath, m.width)
	}

	var parts []string
	for _, b := range m.history {
		parts = append(parts, renderBubble(b, m.width))
	}
	return strings.Join(parts, "\n\n")
}

// renderBubble 根据 bubbleKind 选择对应样式。
//
// 缩进统一使用 lipgloss PaddingLeft 而非手动拼接空格，
// 避免 lipgloss 宽度计算时因 ANSI 转义码与 ASCII 空格混用导致的乱码。
func renderBubble(b bubble, width int) string {
	switch b.kind {
	case bubbleUser:
		return styles.userLabel.Render("▶ "+b.header) + "\n" +
			styles.userContent.PaddingLeft(2).Render(b.content)

	case bubbleAssistant:
		return styles.assistantLabel.Render("◆ "+b.header) + "\n" +
			styles.assistantText.PaddingLeft(2).Render(b.content)

	case bubbleToolCall:
		head := styles.toolCallLabel.Render("▷ " + b.header)
		body := styles.toolCallText.PaddingLeft(2).Render(b.content)
		return head + "\n" + body

	case bubbleToolResult:
		head := styles.toolCallLabel.Render("◀ " + b.header)
		body := styles.toolResultText.PaddingLeft(2).Render(
			truncateLong(b.content, maxToolResult(width)),
		)
		return head + "\n" + body

	case bubbleError:
		return styles.errorLabel.Render("✖ "+b.header) + "\n" +
			styles.errorText.PaddingLeft(2).Render(b.content)

	case bubbleInfo:
		return styles.hintText.Render("ℹ " + b.content)
	}
	return b.content
}

// truncateLong 在工具结果过长时做头尾保留，防止 viewport 内容过长、滚动负担过重。
func truncateLong(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	half := max / 2
	return s[:half] + "\n\n... [已截断] ...\n\n" + s[len(s)-half:]
}

// maxToolResult 随宽度动态调节工具结果的渲染字符上限。
func maxToolResult(width int) int {
	// 经验值：宽度 * 60，保证 1080p 终端下约能展示 ~6000 字符的 stdout。
	// 更多的内容走截断，避免 viewport 渲染卡顿。
	if width <= 0 {
		width = 100
	}
	return width * 60
}

// shortPath 把工作目录过长时只保留末尾两级，便于状态栏不被撑爆。
func shortPath(p string) string {
	const limit = 40
	if len(p) <= limit {
		return p
	}
	parts := strings.Split(p, "/")
	if len(parts) < 3 {
		return "..." + p[len(p)-limit+3:]
	}
	tail := parts[len(parts)-2:]
	return ".../" + strings.Join(tail, "/")
}

// indent 给多行文本整体缩进 n 个空格。
// 注意：此函数仅供测试兼容保留，渲染层已改用 lipgloss PaddingLeft。
func indent(s string, n int) string {
	if s == "" {
		return ""
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}
