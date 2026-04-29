package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version 相关信息可通过 -ldflags "-X tinycode/internal/cli.Version=..." 注入。
// 默认值采用占位符，便于开发期识别未注入的构建。
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// newVersionCmd 返回 `tinycode version` 子命令，打印版本与构建元信息。
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印版本与构建信息",
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "tinycode %s\n", Version)
			fmt.Fprintf(out, "commit:    %s\n", Commit)
			fmt.Fprintf(out, "built:     %s\n", BuildDate)
			fmt.Fprintf(out, "go:        %s\n", runtime.Version())
			fmt.Fprintf(out, "platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
