package tui

import "github.com/charmbracelet/lipgloss"

// styles 汇集 TUI 全部渲染样式。把"外观"与"逻辑"解耦在这里。
// 后续要换配色 / 适配终端主题，只改本文件。
var styles = struct {
	// 外框 & 状态栏
	statusBar  lipgloss.Style
	statusKey  lipgloss.Style
	statusVal  lipgloss.Style
	statusBusy lipgloss.Style
	statusCopy lipgloss.Style // 复制成功反馈

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

	// 欢迎面板
	welcomeBorder   lipgloss.Style // 圆角边框颜色
	welcomeTitle    lipgloss.Style // 嵌入顶边的标题
	welcomeGreeting lipgloss.Style // "Welcome back!" 大字
	welcomeLogo     lipgloss.Style // 品牌图形
	welcomeBrand    lipgloss.Style // tinyCode 品牌名
	welcomeTag      lipgloss.Style // 副标题 "A Tiny Coding Agent"
	welcomeLabel    lipgloss.Style // 栏目标题（"快速开始"/"提示"）
	welcomeKey      lipgloss.Style // 按键名
	welcomeDesc     lipgloss.Style // 描述文字
	welcomeField    lipgloss.Style // 字段名（模型/目录）
	welcomeValue    lipgloss.Style // 字段值
	welcomeDim      lipgloss.Style // 暗色文字
	welcomeRule     lipgloss.Style // 分隔线
	welcomeBullet   lipgloss.Style // 列表前缀符号
}{
	statusBar: lipgloss.NewStyle().
		Background(lipgloss.Color("67")). // 钢蓝 SteelBlue3，与欢迎面板边框统一
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
	statusCopy: lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
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
		Foreground(lipgloss.Color("178")). // 金黄 LightGoldenrod3，表示工具执行中
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

	welcomeBorder: lipgloss.NewStyle().
		Foreground(lipgloss.Color("67")), // 钢蓝 SteelBlue3，柔和不刺眼
	welcomeTitle: lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true),
	welcomeGreeting: lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Bold(true),
	welcomeLogo: lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true),
	welcomeBrand: lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true),
	welcomeTag: lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true),
	welcomeLabel: lipgloss.NewStyle().
		Foreground(lipgloss.Color("111")). // SkyBlue2，柔和淺蓝
		Bold(true),
	welcomeKey: lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true),
	welcomeDesc: lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")),
	welcomeField: lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")),
	welcomeValue: lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true),
	welcomeDim: lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")),
	welcomeRule: lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")),
	welcomeBullet: lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")), // 绿色 SpringGreen2，状态指示灯
}
