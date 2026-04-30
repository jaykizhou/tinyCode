package cli

import (
	"github.com/spf13/cobra"

	"tinycode/internal/cli/config"
	"tinycode/internal/ui/repl"
)

// newReplCmd 返回 `tinycode repl` 子命令。
//
// 该命令对应旧入口的纯文本循环体验，
// 适合 CI / 调试 / 无 TTY 场景，以及用户不希望进入 alt-screen TUI 时使用。
func newReplCmd(cfg *config.RuntimeConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "repl",
		Short: "以纯文本 REPL 的方式与 Agent 交互",
		Long:  "纯文本 REPL：一行输入 → Agent 一次 RunLoop → 打印结果，适合 CI / 无 TTY 场景。",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Finalize(cmd.Flags()); err != nil {
				return err
			}
			return repl.Run(cmd.Context(), *cfg)
		},
	}
}
