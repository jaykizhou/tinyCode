// Package repl 提供纯文本 REPL 实现。
//
// 它保留旧入口的体验：一行输入、一次 Agent.RunLoop、打印结果。
// 与 TUI 的差异仅在于"渲染通道"：REPL 直接写 stdout / stderr，
// 而 Agent 的装配路径（bootstrap）完全一致。
package repl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"tinycode/internal/agent"
	"tinycode/internal/cli/bootstrap"
	"tinycode/internal/cli/config"
)

// Run 阻塞式执行 REPL，直到用户输入 quit/exit/:q 或关闭 stdin。
//
// ctx 来自 cobra：对应 Ctrl+C / SIGTERM 时 cobra 已经注入的取消信号，
// 这里再包一层 signal.NotifyContext 主要是为了在非 cobra 入口下仍能工作。
func Run(ctx context.Context, cfg config.RuntimeConfig) error {
	a, err := bootstrap.Build(cfg, bootstrap.Options{
		ExtraAgentOptions: []agent.Option{
			agent.WithLogger(func(event string, kv ...any) {
				if cfg.Verbose {
					fmt.Fprintf(os.Stderr, "[agent] %s %v\n", event, kv)
				}
			}),
			agent.WithEventSink(newStderrSink(cfg.Verbose)),
		},
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	printBanner(cfg)

	scanner := bufio.NewScanner(os.Stdin)
	// 默认 64KB 对多行粘贴偏小，这里把上限抬到 1MB
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		fmt.Print("\n你> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "quit" || input == "exit" || input == ":q" {
			fmt.Println("再见！")
			return nil
		}

		reply, err := a.RunLoop(ctx, input)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Fprintln(os.Stderr, "[已取消]")
				return nil
			}
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			continue
		}
		fmt.Printf("\nAgent> %s\n", reply)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取输入失败: %w", err)
	}
	return nil
}

// printBanner 输出启动横幅。
func printBanner(cfg config.RuntimeConfig) {
	fmt.Println("===================================")
	fmt.Println(" TinyCode REPL (one-loop minimal)  ")
	fmt.Println("===================================")
	fmt.Printf("模型: %s\n", cfg.Model)
	fmt.Printf("工作目录: %s\n", cfg.WorkDir)
	fmt.Println("输入 quit / exit / :q 退出。")
}

// stderrSink 把结构化事件转换为 stderr 文本。
//
// 在 verbose=false 时仅输出工具调用相关事件，保持 stdout 清爽；
// verbose=true 时把所有事件打印出来，便于排查。
type stderrSink struct {
	verbose bool
}

func newStderrSink(verbose bool) *stderrSink { return &stderrSink{verbose: verbose} }

// Emit 实现 agent.EventSink。
func (s *stderrSink) Emit(e agent.Event) {
	switch e.Kind {
	case agent.EventToolCall:
		fmt.Fprintf(os.Stderr, "  [tool-call] %s %s\n", e.ToolName, truncate(string(e.Args), 120))
	case agent.EventToolResult:
		fmt.Fprintf(os.Stderr, "  [tool-result] %s: %s\n", e.ToolName, truncate(e.Payload, 200))
	case agent.EventError:
		fmt.Fprintf(os.Stderr, "  [error] %s\n", e.Payload)
	default:
		if s.verbose {
			fmt.Fprintf(os.Stderr, "  [%s] iter=%d %s\n", e.Kind, e.Iter, truncate(e.Payload, 120))
		}
	}
}

// truncate 限制单行日志长度，避免工具输出爆屏。
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
