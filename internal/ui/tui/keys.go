package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap 集中声明全部快捷键。把键位与行为解耦在这里，方便后续做 help 面板或用户自定义。
type keyMap struct {
	Submit     key.Binding
	Newline    key.Binding
	Quit       key.Binding
	ClearHist  key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	Cancel     key.Binding
	Copy       key.Binding
}

// defaultKeyMap 返回项目默认的键位表。
func defaultKeyMap() keyMap {
	return keyMap{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "发送"),
		),
		Newline: key.NewBinding(
			key.WithKeys("shift+enter", "alt+enter", "ctrl+j"),
			key.WithHelp("shift+enter", "换行"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "退出"),
		),
		ClearHist: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "清屏"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "上翻"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "下翻"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "取消/退出"),
		),
		Copy: key.NewBinding(
			key.WithKeys("ctrl+y"),
			key.WithHelp("ctrl+y", "复制对话内容"),
		),
	}
}
