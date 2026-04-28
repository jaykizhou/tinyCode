// Package shell 提供一个跨平台的系统命令执行工具。
//
// 设计取舍见 one_loop.md 5.6：一个 Shell 工具足以让模型完成绝大多数编程任务。
// 与文档不同之处在于：为了同时支持 Windows，我们根据 runtime.GOOS 选择底层解释器——
//
//	windows  -> powershell -NoProfile -Command
//	其它(类 Unix) -> bash -c
package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"tinycode/internal/agent"
)

// 常量：超时、输出截断、默认参数 Schema。
const (
	defaultTimeout = 30 * time.Second // 默认命令超时
	maxOutputBytes = 50000            // 单次输出上限，超过则头尾截断，防止 token 爆炸
)

// parametersSchema 是工具参数 JSON Schema。
// 使用明确而具体的描述，帮助模型决定什么时候以及如何调用。
var parametersSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "要执行的 shell 命令。Windows 上使用 PowerShell 语法，其它系统使用 bash 语法。"
    },
    "timeout": {
      "type": "integer",
      "description": "命令超时时间（秒），默认 30 秒。"
    }
  },
  "required": ["command"]
}`)

// Input 对应模型传入的参数。
type Input struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// Tool 实现 agent.Tool 接口。
type Tool struct {
	workDir        string        // 命令执行的工作目录
	defaultTimeout time.Duration // 默认超时
	blacklist      []string      // 危险命令黑名单
}

// New 创建一个 Shell 工具实例。
// workDir 为空时将继承调用方进程的当前目录。
func New(workDir string) *Tool {
	return &Tool{
		workDir:        workDir,
		defaultTimeout: defaultTimeout,
		blacklist:      defaultBlacklist(),
	}
}

// 以下四个方法实现 agent.Tool 接口。

func (t *Tool) Name() string { return "shell" }

func (t *Tool) Description() string {
	return "在本机 shell 中执行命令。Windows 下使用 PowerShell，其它系统使用 bash。" +
		"用于读写文件、运行程序、查询系统信息等，执行结果以文本返回。"
}

func (t *Tool) Parameters() json.RawMessage { return parametersSchema }

// Execute 是工具的核心逻辑：
//  1. 参数反序列化；
//  2. 黑名单拦截；
//  3. 选择解释器 + 应用超时；
//  4. 运行命令并合并 stdout/stderr；
//  5. 输出过长时头尾截断。
func (t *Tool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", fmt.Errorf("参数反序列化失败: %w", err)
	}
	if input.Command == "" {
		return "", fmt.Errorf("参数 command 不能为空")
	}

	// 1) 黑名单拦截——注意这里不返回 Go error，而是返回给模型一条文本提示，
	//    让它可以换一种方式继续尝试。
	if reason, blocked := t.isBlocked(input.Command); blocked {
		return fmt.Sprintf("命令被安全策略拦截：%s\n请尝试使用更小范围的命令。", reason), nil
	}

	// 2) 应用超时
	timeout := t.defaultTimeout
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3) 根据操作系统选择解释器
	name, args := pickShell(input.Command)
	cmd := exec.CommandContext(execCtx, name, args...)
	if t.workDir != "" {
		cmd.Dir = t.workDir
	}

	// 4) 合并 stdout 与 stderr，一次性读取
	output, runErr := cmd.CombinedOutput()
	result := string(output)

	// 5) 超长输出头尾截断，中间省略
	if len(result) > maxOutputBytes {
		half := maxOutputBytes / 2
		result = result[:half] +
			"\n\n... [输出已截断，中间省略] ...\n\n" +
			result[len(result)-half:]
	}

	// 如果是因为超时被终止，给模型一个明确提示
	if execCtx.Err() == context.DeadlineExceeded {
		return result + "\n[命令超时]", nil
	}
	// 其它执行错误（非 0 退出码等）不返回 Go error，
	// 而是把错误信息贴到输出末尾，让模型能看到并自行修复。
	if runErr != nil {
		return fmt.Sprintf("%s\n[退出码/错误: %v]", result, runErr), nil
	}
	return result, nil
}

// pickShell 按平台选择解释器与参数。
// 单独抽出便于后续替换（例如允许用户显式指定 /bin/sh 或 cmd.exe）。
func pickShell(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		// -NoProfile：跳过 $PROFILE 启动脚本，加快启动并避免环境污染；
		// -Command：作为单次执行，不进入交互模式。
		return "powershell", []string{"-NoProfile", "-Command", command}
	}
	return "bash", []string{"-c", command}
}

// 编译期断言：Tool 必须满足 agent.Tool 接口。
var _ agent.Tool = (*Tool)(nil)
