// Command tinycode 是 TinyCode 的 CLI 入口。
//
// 本文件刻意保持极简：仅负责构造 Cobra root 命令并驱动其 Execute，
// 所有业务逻辑（TUI / REPL / 装配 / 配置）都在 internal 子包内。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"tinycode/internal/cli"
)

func main() {
	// 用 NotifyContext 把 SIGINT/SIGTERM 转为 context 取消，传给 Cobra 与 TUI/REPL 共用。
	// TUI 会在内部接管 Ctrl+C 键位；REPL 仍然使用信号取消语义。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.NewRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
