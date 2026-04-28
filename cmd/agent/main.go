// Command agent 是 TinyCode 最小 Agent 的命令行入口。
//
// 它的职责非常单一：
//  1. 读取环境变量 / 命令行参数，装配模型与工具；
//  2. 启动一个简易的 REPL 交互循环；
//  3. 把每轮用户输入交给 Agent.RunLoop，并把结果打印回终端。
//
// 整个文件刻意保持扁平，没有任何状态机 / 路由——这正是 one_loop.md 所倡导的：
// 智能留给模型，Harness 只做忠实的 I/O。
package main

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
	"tinycode/internal/model/openai"
	"tinycode/internal/tools/shell"
)

// 环境变量键名。与 OpenAI 官方约定对齐，便于复用通用工具链的 API Key 配置。
const (
	envAPIKey  = "OPENAI_API_KEY"
	envBaseURL = "OPENAI_BASE_URL"
	envModel   = "OPENAI_MODEL"
)

// 默认配置。
const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "gpt-4o-mini"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

// run 是 main 的"可测试版本"：不直接 os.Exit，便于将来加入集成测试。
func run() error {
	// 1) 读取配置
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return fmt.Errorf("请先设置环境变量 %s", envAPIKey)
	}
	baseURL := firstNonEmpty(os.Getenv(envBaseURL), defaultBaseURL)
	modelName := firstNonEmpty(os.Getenv(envModel), defaultModel)

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %w", err)
	}

	// 2) 装配 Agent：模型 + Shell 工具
	client := openai.NewClient(baseURL, apiKey, modelName)
	shellTool := shell.New(workDir)

	a, err := agent.NewAgent(
		agent.WithModel(client),
		agent.WithTools(shellTool),
		agent.WithMaxIterations(25),
		// 把关键事件写到 stderr，方便观察工具调用过程；
		// 输出本身不会污染 stdout 上的模型回复。
		agent.WithLogger(func(event string, kv ...any) {
			fmt.Fprintf(os.Stderr, "[agent] %s %v\n", event, kv)
		}),
	)
	if err != nil {
		return fmt.Errorf("装配 Agent 失败: %w", err)
	}

	// 3) 监听 Ctrl+C，把信号转换为 context 取消，可以中断正在执行的长命令
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	printBanner(modelName, workDir)

	// 4) REPL 循环：一行输入 → 一次 Agent 调用 → 打印结果
	scanner := bufio.NewScanner(os.Stdin)
	// 默认的 64KB 缓冲对多行粘贴偏小，这里把上限抬到 1MB
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
			// 区分"用户主动取消"和"其它错误"，取消时优雅退出
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

// printBanner 输出启动横幅。把关键配置打出来，方便排查问题。
func printBanner(model, workDir string) {
	fmt.Println("===================================")
	fmt.Println(" TinyCode Agent (one-loop minimal) ")
	fmt.Println("===================================")
	fmt.Printf("模型: %s\n", model)
	fmt.Printf("工作目录: %s\n", workDir)
	fmt.Println("输入 quit / exit / :q 退出。")
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
