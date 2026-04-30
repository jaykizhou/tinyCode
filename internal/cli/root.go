// Package cli 汇总 Cobra 命令层：root 命令、子命令与标志绑定。
//
// 设计上保持与 UI 实现解耦：root 默认进入 TUI；子命令 repl / version 分别处理纯文本循环与版本信息。
// 新增子命令只需在本目录下加文件，并通过 AddCommand 挂到 root，即可复用同一份 RuntimeConfig。
package cli

import (
	"github.com/spf13/cobra"

	"tinycode/internal/cli/config"
	"tinycode/internal/ui/tui"
)

// NewRootCmd 构造 tinycode 的根命令。
//
// 行为约定：
//   - 不带子命令时启动 Bubble Tea TUI；
//   - PersistentFlags 上绑定所有运行时配置，所有子命令共享；
//   - SilenceUsage/SilenceErrors 避免 cobra 在错误时重复打印使用说明。
func NewRootCmd() *cobra.Command {
	cfg := &config.RuntimeConfig{}

	cmd := &cobra.Command{
		Use:           "tinycode",
		Short:         "最小可用的 one-loop Coding Agent",
		Long:          "TinyCode 是一个以 one-loop 为核心思想的轻量 Coding Agent，默认启动 Bubble Tea TUI。",
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Finalize(cmd.Flags()); err != nil {
				return err
			}
			return tui.Run(cmd.Context(), *cfg)
		},
	}

	config.BindFlags(cmd.PersistentFlags(), cfg)

	cmd.AddCommand(newReplCmd(cfg))
	cmd.AddCommand(newVersionCmd())
	return cmd
}
