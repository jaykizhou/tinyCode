# TinyCode 最小 Agent 实现计划

## 1. 设计目标与核心哲学

- 忠实实现 `one_loop.md` 的 "Model + Harness" 思想：Harness 尽量薄、只做忠实转发，不写编排逻辑
- 对齐 blades-main 的工程实践：接口抽象 + Functional Options + 分层目录 + 显式错误类型
- 模型接入参考 `d:\workspace\go-project\tinyCode\.qoder\example\openai\client.go`，使用 OpenAI 兼容协议（chat/completions + tools）
- 最小化范围：一个核心循环 + 一个跨平台 Shell 工具，但接口预留扩展位

## 2. 目标目录结构

```
tinyCode/
├── cmd/agent/main.go                  # CLI 入口：REPL 交互，装配 Agent
├── internal/
│   ├── agent/
│   │   ├── types.go                   # Message / Content / ToolCall / CompletionRequest 等
│   │   ├── conversation.go            # Conversation：messages 只追加不修改
│   │   ├── model.go                   # Model 接口定义（对齐 openai 示例）
│   │   ├── options.go                 # WithModel / WithSystemPrompt / WithTools / WithMaxIterations
│   │   ├── agent.go                   # Agent 核心结构 + RunLoop 30 行核心循环
│   │   └── errors.go                  # 统一错误：ErrMaxIterations / ErrModelRequired ...
│   ├── model/openai/
│   │   └── client.go                  # OpenAI 兼容 HTTP 客户端，实现 agent.Model
│   └── tools/
│       ├── tool.go                    # Tool 接口 + Registry（dispatch map）
│       └── shell/
│           ├── shell.go               # 跨平台 Shell 工具：Windows→powershell；其它→bash
│           └── blacklist.go           # 危险命令黑名单
├── docs/DESIGN.md                     # 架构与实现说明文档
├── go.mod / go.sum
└── README.md                          # 快速开始（保留现有 README，仅补充运行说明）
```

## 3. 关键类型与接口设计

### 3.1 内部消息模型（`internal/agent/types.go`）

对齐 openai 示例的中性抽象，与具体 API 解耦：

```go
type Message struct {
    Role       string     // "system" | "user" | "assistant" | "tool"
    Content    string     // 文本内容
    ToolCallID string     // tool 消息关联的调用 ID
    ToolCalls  []ToolCall // assistant 发起的工具调用
}

type ToolCall struct {
    ID        string
    Name      string
    Arguments json.RawMessage
}

type ToolDefinition struct {
    Name        string
    Description string
    Parameters  json.RawMessage // JSON Schema
}

type CompletionRequest struct {
    SystemPrompt string
    Messages     []Message
    Tools        []ToolDefinition
}

type CompletionResponse struct {
    Message   Message
    ToolCalls []ToolCall
    Stop      string // "stop" | "tool_calls"
}
```

### 3.2 Model 接口（`internal/agent/model.go`）

```go
type Model interface {
    Name() string
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```

OpenAI 客户端 `internal/model/openai/client.go` 实现该接口（直接复用示例 `client.go` 的核心逻辑，仅调整包路径和增加中文注释）。

### 3.3 Tool 接口与注册表（`internal/tools/tool.go`）

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}

type Registry struct { tools map[string]Tool }
func (r *Registry) Register(t Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) Definitions() []agent.ToolDefinition
```

### 3.4 Conversation（`internal/agent/conversation.go`）

严格执行 one_loop.md 5.3 的 "只追加不修改" 规则：

```go
type Conversation struct { messages []Message }
func (c *Conversation) Append(m Message)
func (c *Conversation) Snapshot() []Message // 返回副本
```

### 3.5 Agent 核心循环（`internal/agent/agent.go`）

核心 `RunLoop` 遵循 one_loop.md 5.5 的 30 行骨架，但工具结果用 `role: "tool"` 追加（OpenAI 协议规范）：

```go
func (a *Agent) RunLoop(ctx context.Context, userInput string) (string, error) {
    a.conv.Append(Message{Role: "user", Content: userInput})
    for i := 0; i < a.maxIterations; i++ {
        resp, err := a.model.Complete(ctx, CompletionRequest{
            SystemPrompt: a.systemPrompt,
            Messages:     a.conv.Snapshot(),
            Tools:        a.registry.Definitions(),
        })
        if err != nil { return "", err }
        a.conv.Append(Message{Role: "assistant", Content: resp.Message.Content, ToolCalls: resp.ToolCalls})
        if len(resp.ToolCalls) == 0 { return resp.Message.Content, nil }
        for _, call := range resp.ToolCalls {
            out := a.execute(ctx, call)
            a.conv.Append(Message{Role: "tool", ToolCallID: call.ID, Content: out})
        }
    }
    return "", ErrMaxIterations
}
```

采用 blades-main 风格的 Functional Options 装配：

```go
func NewAgent(opts ...Option) (*Agent, error)
WithModel(m Model)
WithSystemPrompt(s string)
WithTools(ts ...Tool)
WithMaxIterations(n int)
```

## 4. 跨平台 Shell 工具设计（`internal/tools/shell`）

根据用户选择，实现跨平台适配：

```go
func (s *ShellTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
    // 1. 反序列化参数 {command, timeout}
    // 2. 黑名单匹配拦截（大小写不敏感 + 子串匹配）
    // 3. 依据 runtime.GOOS 选择：
    //      windows -> exec.Command("powershell", "-NoProfile", "-Command", cmd)
    //      其它     -> exec.Command("bash", "-c", cmd)
    // 4. 使用 context.WithTimeout 控制超时（默认 30s）
    // 5. CombinedOutput 获取合并输出，超过 50000 字节做头尾截断
    // 6. 命令失败不返回 Go error，把错误信息拼入输出返回给模型
}
```

黑名单（`blacklist.go`）保留 one_loop.md 5.7 的清单，并针对 Windows 补充：`Remove-Item -Recurse -Force`、`Format-Volume`、`Stop-Computer`、`Restart-Computer`。

## 5. CLI 入口（`cmd/agent/main.go`）

- 读取环境变量：`OPENAI_API_KEY`、`OPENAI_BASE_URL`（默认 `https://api.openai.com/v1`）、`OPENAI_MODEL`（默认 `gpt-4o-mini`）
- 工作目录取 `os.Getwd()` 传给 Shell 工具
- 使用 `bufio.Scanner` 实现 REPL，支持 `quit`/`exit` 退出
- 捕获 `os.Interrupt` 让 ctx 可取消

## 6. 文档产出（`docs/DESIGN.md`）

一份中文架构设计与实现说明文档，包含：

1. 设计哲学：Model + Harness / One Loop is All You Need
2. 模块划分图（Mermaid `graph TB`，遵守无样式规则）
3. 时序图：一次用户输入 → 多轮工具调用 → 最终回复
4. 核心数据流：`Conversation` 的只追加特性与角色映射表
5. 扩展点：如何新增 Tool、如何替换 Model
6. 安全考虑：黑名单、超时、输出截断
7. 已知局限与后续演进方向（流式、多轮记忆压缩、多模型路由）

## 7. 依赖与构建

- 仅使用 Go 标准库（`net/http`、`encoding/json`、`os/exec`、`context`、`bufio`），不引入第三方依赖
- `go.mod` 模块名定为 `tinycode`，Go 版本与当前工具链一致
- 提供最简运行指令：`go run ./cmd/agent`

## 8. 验证方式

- `go build ./...` 通过编译
- `go vet ./...` 无告警
- 手工冒烟：在设置 API Key 后运行 REPL，输入 "列出当前目录文件"，观察模型调用 shell 工具并返回结果

## 9. 不做的事（范围守则）

- 不实现流式响应（保持最小）
- 不实现会话持久化/压缩（保持最小）
- 不引入中间件链 / 图执行器（blades-main 的 graph/flow 超出本次范围）
- 不编写单元测试（保持最小；接口设计上允许后续补充 mock）
