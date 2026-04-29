package tui

import "github.com/charmbracelet/lipgloss"

// styles 汇集 TUI 全部渲染样式。把"外观"和"逻辑"解耦在这里，
// 后续要换配色 / 适配终端主题，只改本文件。
var styles = struct {
	// 外框 & 状态栏
	statusBar  lipgloss.Style
	statusKey  lipgloss.Style
	statusVal  lipgloss.Style
	statusBusy lipgloss.Style

	// 气泡样式
	userLabel      lipgloss.Style
	userContent    lipgloss.Style
	assistantLabel lipgloss.Style
	assistantText  lipgloss.Style
	toolCallLabel  lipgloss.Style
	toolCallText   lipgloss.Style
	toolResultText lipgloss.Style
	errorLabel     lipgloss.Style
	errorText      lipgloss.Style
	hintText       lipgloss.Style
	separator      lipgloss.Style
}{
	statusBar: lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1),
	statusKey: lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Bold(true),
	statusVal: lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")),
	statusBusy: lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true),

	userLabel: lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true),
	userContent: lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")),

	assistantLabel: lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true),
	assistantText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")),

	toolCallLabel: lipgloss.NewStyle().
		Foreground(lipgloss.Color("213")).
		Bold(true),
	toolCallText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")),
	toolResultText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")),

	errorLabel: lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true),
	errorText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("203")),

	hintText: lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true),
	separator: lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")),
}
