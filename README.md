# tinyCode

> **一个最小可用的 A Tiny Coding Agent** —— 让大模型在本地终端里"活"起来。

---

## 1. 项目简介

### tinyCode 是什么

**tinyCode** 是一个轻量级的 Coding Agent。它的核心使命很简单：你在终端里输入一句话，Agent 自动决定是否需要调用工具（比如执行 Shell 命令），再把结果汇总成最终回复给你。

市面上大多数 Agent 框架为了"可扩展"而变得臃肿，中间件链、图执行器、插件系统层层叠加，反而让人看不清"模型到底做了什么"。tinyCode 反其道而行，用不到 1500 行代码证明：**一个 for 循环 + 两个接口，足以支撑 90% 的编程助手场景。**

### 核心特点

- **极简架构**：没有中间件链，没有状态机，只有一个干净的 `RunLoop`
- **工具调用**：内置 Shell 工具，模型可自动执行命令、读写文件、运行程序
- **安全可控**：命令黑名单拦截危险操作，输出截断防止 Token 爆炸
- **双 UI 模式**：默认启动漂亮的 TUI 界面，也支持纯文本 REPL 模式
- **接口解耦**：模型、工具、UI 全部通过接口交互，任意组件可替换

### 技术栈概览

| 技术/库 | 版本 | 用途 |
|---------|------|------|
| Go | 1.25 | 全栈语言，零运行时依赖 |
| Cobra | v1.10.2 | CLI 命令树与参数解析 |
| Bubble Tea | v1.3.10 | 交互式终端 TUI |
| Lipgloss | v1.1.0 | 终端样式渲染 |
| OpenAI API | 兼容端点 | 大模型对话与工具调用 |

---

## 2. 功能特性

### Agent 智能对话

tinyCode 的核心是一个**one-loop Agent 引擎**。它的工作方式非常直观：

1. 你输入一句话（比如"列出当前目录的文件"）
2. Agent 把这句话发给大模型
3. 大模型判断是否需要调用工具
4. 如果需要，Agent 执行工具并把结果回写给模型
5. 模型基于工具结果给出最终回复

整个过程对用户是透明的——你只需要像聊天一样输入问题，Agent 会自动处理背后的所有调用链。

### Shell 工具调用

tinyCode 内置了一个跨平台的 **Shell 工具**，让模型能够在你的本地环境中执行命令：

- **类 Unix 系统**（macOS / Linux）：使用 `bash -c`
- **Windows**：使用 `powershell -NoProfile -Command`

模型可以借助这个工具完成绝大多数编程任务：查看文件内容、编译代码、运行测试、查询系统信息等。

### 安全控制（命令黑名单）

为了防止模型误执行危险命令，tinyCode 内置了**命令黑名单**机制，采用子串匹配覆盖常见变体：

- `rm -rf /`、`mkfs`、`> /dev/sda`
- `git push --force`、`git reset --hard`
- Windows 下的 `remove-item -recurse -force`、`format-volume`

命中黑名单时，命令不会执行，而是返回一条提示给模型，让它有机会换一种方式继续尝试。

### 双 UI 模式（TUI + REPL）

| 模式 | 命令 | 适用场景 |
|------|------|----------|
| **TUI**（默认） | `tinycode` | 日常交互，界面美观，支持实时事件气泡 |
| **REPL** | `tinycode repl` | CI / 调试 / 无 TTY 场景，纯文本输入输出 |

---

## 3. 快速开始

### 环境要求

- **Go 1.25 或更高版本**
- 一个 **OpenAI API Key**（或任意兼容 OpenAI 协议的 API 端点）

### 安装方式

**方式一：通过 `go install` 安装**

```bash
go install github.com/yourname/tinycode/cmd/tinycode@latest
```

**方式二：克隆后本地构建**

```bash
git clone https://github.com/yourname/tinycode.git
cd tinycode
go build -o tinycode ./cmd/tinycode
```

> 构建时可通过 `-ldflags` 注入版本信息：
> ```bash
> go build -ldflags "-X tinycode/internal/cli.Version=1.0.0" -o tinycode ./cmd/tinycode
> ```

### 配置说明

tinyCode 支持 **三种配置来源**，按优先级从高到低依次叠加，下层仅在上层未提供时生效：

**CLI Flag > 环境变量 > `config.yaml` > 内置默认值**

| 字段 | CLI flag | 环境变量 | 配置文件键 | 默认值 |
|------|----------|----------|------------|--------|
| API Key | `--api-key` | `OPENAI_API_KEY` | `api_key` | — （必填） |
| Base URL | `--base-url` | `OPENAI_BASE_URL` | `base_url` | `https://api.openai.com/v1` |
| 模型名 | `--model` | `OPENAI_MODEL` | `model` | `gpt-4o-mini` |
| 观测开关 | `--trace` | `TINYCODE_TRACE` | `trace` | `false` |
| 观测目录 | `--trace-dir` | `TINYCODE_TRACE_DIR` | `trace_dir` | `.tinycode/trace` |

> 工作目录 `--work-dir`、最大迭代数 `--max-iter` 等其他字段仍只从 flag/环境变量读取，不进入配置文件。

**方式一：环境变量（推荐）**

```bash
export OPENAI_API_KEY="sk-xxxxxxxxxxxxxxxxxxxxxxxx"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export OPENAI_MODEL="gpt-4o-mini"
```

**方式二：`config.yaml` 文件**

在运行目录放置一个 `config.yaml`（或通过 `--config /path/to/file.yaml` 指定）：

```yaml
# tinyCode 运行配置（所有字段可选）
api_key: sk-xxxxxxxxxxxxxxxxxxxxxxxx
base_url: https://api.openai.com/v1
model: gpt-4o-mini

# 可选：开启模型交互观测，将每次 request/response/error 写入 JSONL 日志
trace: true
trace_dir: .tinycode/trace
```

- 文件不存在时静默忽略，不会报错；
- 未知字段会被忽略，便于向前兼容；
- 支持行末 `#` 注释与单/双引号包裹的值。

配置完成后，直接运行 `tinycode` 即可启动 TUI 界面：

```bash
tinycode
```

---

## 4. 使用说明

### 基本命令

```bash
# 默认启动 TUI 模式
tinycode

# 启动纯文本 REPL 模式
tinycode repl

# 查看版本与构建信息
tinycode version
```

### 命令行参数说明

所有参数在 root 命令上通过 **PersistentFlags** 定义一次，子命令自动继承。

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--api-key` | — | 读取 `OPENAI_API_KEY` / `config.yaml` | API Key；可通过环境变量或配置文件提供 |
| `--base-url` | — | `https://api.openai.com/v1` | OpenAI 兼容 API 的 Base URL |
| `--model` | — | `gpt-4o-mini` | 模型名称 |
| `--config` | — | `config.yaml` | YAML 配置文件路径；文件不存在时忽略 |
| `--work-dir` | — | 当前目录 | Shell 工具的工作目录 |
| `--max-iter` | — | `25` | Agent 主循环最大迭代次数 |
| `--system` | — | 内置提示词 | 覆盖默认系统提示词 |
| `--verbose` | `-v` | `false` | 输出工具调用等详细日志 |
| `--trace` | — | `false` | 开启模型交互观测，将 HTTP request/response 写入 JSONL 日志 |
| `--trace-dir` | — | `.tinycode/trace` | 观测日志输出目录（开启 `--trace` 时生效） |

**使用示例：**

```bash
# 指定模型和工作目录
tinycode --model gpt-4o --work-dir /path/to/project

# REPL 模式 + 详细日志
tinycode repl -v

# 自定义系统提示词
tinycode --system "你是一个专注 Go 语言的专家"
```

### TUI 模式操作说明

启动 `tinycode`（不带子命令）后，你会进入一个基于 Bubble Tea 的交互式终端界面：

- **输入框**位于底部，直接打字输入你的问题
- 按 **Enter** 发送消息给 Agent
- 按 **Ctrl+C** 执行**二段式退出**：
  - 如果 Agent 正在思考 / 调用工具 → **取消当前对话**，程序保持运行
  - 如果处于空闲状态 → **退出程序**
- TUI 会实时显示对话气泡，包括模型的回复和工具调用的中间过程

### REPL 模式操作说明

启动 `tinycode repl` 后，进入纯文本交互模式：

```
===================================
 TinyCode REPL (one-loop minimal)
===================================
模型: gpt-4o-mini
工作目录: /Users/you/project
输入 quit / exit / :q 退出。

你> 帮我查看 go.mod 的内容
Agent> 当前项目的 go.mod 内容如下：...

你> quit
再见！
```

- 每行输入会触发一次完整的 Agent `RunLoop`
- 输入 `quit`、`exit` 或 `:q` 退出程序
- 按 **Ctrl+C** 可取消当前正在进行的对话
- 加 `-v` 参数可以查看工具调用的详细日志

### 模型交互观测（排查错误利器）

当遇到 `status=400` / `status=500` / 超时 / 模型格式不匹配等报错时，tinyCode 可以将与大模型 API 的每次 HTTP 交互原样记录下来，便于事后回放。开启方式：

```bash
# 方式 1：CLI flag
tinycode --trace
tinycode --trace --trace-dir ./debug-logs

# 方式 2：环境变量
export TINYCODE_TRACE=1
export TINYCODE_TRACE_DIR=./debug-logs   # 可选

# 方式 3：config.yaml
# trace: true
# trace_dir: .tinycode/trace
```

开启后，每次运行会在观测目录生成一个以时间戳命名的 JSONL 文件，例如：

```
.tinycode/trace/openai-20260430-150230.jsonl
```

文件内容为每行一个 JSON 记录，`kind` 字段有三种取值：

| kind | 含义 | 关键字段 |
|------|------|----------|
| `request` | 请求发出前 | `url` / `headers` / `payload` |
| `response` | 收到响应（包括非 2xx） | `status` / `body` / `duration_ms` |
| `error` | 传输层错误 | `error` / `duration_ms` |

> `Authorization` 等包含凭证的请求头会自动替换为 `***redacted***`，避免 API Key 泄露。

典型排查命令（PowerShell）：

```powershell
# 看上一次运行中所有非 2xx 响应
Get-Content .\.tinycode\trace\openai-*.jsonl |
    ConvertFrom-Json |
    Where-Object { $_.status -ge 400 }
```

---

## 5. 项目结构

```
tinyCode/
├── cmd/
│   └── tinycode/
│       └── main.go              # 入口：极简，只构造 root 命令
├── internal/
│   ├── agent/
│   │   ├── types.go             # 中性消息模型（Message / ToolCall）
│   │   ├── model.go             # Model 接口：大模型最小契约
│   │   ├── tool.go              # Tool 接口 + Registry 注册表
│   │   ├── conversation.go      # 只追加的会话历史（线程安全）
│   │   ├── agent.go             # RunLoop 主循环（核心引擎）
│   │   ├── options.go           # Functional Options 装配
│   │   ├── event.go             # 结构化事件协议
│   │   └── errors.go            # 可识别错误类型
│   ├── model/openai/
│   │   ├── client.go            # OpenAI 兼容 HTTP 客户端
│   │   └── observer.go          # 模型交互观测器（JSONL 日志）
│   ├── tools/shell/
│   │   ├── shell.go             # 跨平台 Shell 工具
│   │   └── blacklist.go         # 危险命令黑名单
│   ├── cli/
│   │   ├── root.go              # Cobra 根命令（默认进入 TUI）
│   │   ├── repl_cmd.go          # repl 子命令
│   │   ├── version_cmd.go       # version 子命令
│   │   ├── config/
│   │   │   ├── config.go        # 四层配置优先级合并（flag/env/file/default）
│   │   │   └── file.go          # config.yaml 最小 YAML 解析器
│   │   └── bootstrap/
│   │       └── bootstrap.go     # Agent 装配工厂
│   └── ui/
│       ├── repl/
│       │   └── repl.go          # 纯文本 REPL 实现
│       └── tui/                 # Bubble Tea TUI（7 文件拆分）
│           ├── program.go         # 程序入口与启动
│           ├── model.go           # 状态定义
│           ├── update.go          # 消息路由与状态更新
│           ├── view.go            # 界面渲染
│           ├── runner.go          # 异步任务包装
│           ├── events.go          # channelSink 事件消费
│           ├── styles.go          # Lipgloss 样式
│           └── keys.go            # 快捷键绑定
├── docs/
│   ├── DESIGN.md                # 架构设计文档
│   ├── PLAN.md                  # 实现计划
│   └── TECHNICAL.md             # 技术实现详解
├── go.mod
├── go.sum
└── README.md
```

---

## 6. 架构概览

### 分层说明

tinyCode 采用清晰的分层架构，每一层只向下依赖一层，通过**接口**而非具体实现交互：

| 层级 | 职责 | 核心设计原则 |
|------|------|-------------|
| **cmd** | 程序入口 | 只构造命令、驱动 Execute，零业务逻辑 |
| **cli** | 命令树、配置解析、对象装配 | Cobra 管路由，bootstrap 管装配，职责分离 |
| **agent** | Agent 引擎：RunLoop、会话、工具注册、事件 | Harness 尽量薄，模型做决策 |
| **model / tools** | 大模型与工具的具体实现 | 通过接口接入，可替换 |
| **ui** | 用户交互层：TUI / REPL | 消费事件而非侵入 Agent |

数据流向大致如下：

```
用户输入 → CLI 层解析配置 → bootstrap 装配 Agent → Agent.RunLoop
                                                      ↓
                                              调用 Model（大模型）
                                                      ↓
                                              需要工具？→ Registry 查找并执行
                                                      ↓
                                              返回最终结果
                                                      ↓
                                              UI 层渲染展示
```

> 更详细的架构分析请参阅 [`docs/TECHNICAL.md`](docs/TECHNICAL.md)。

### 核心设计理念

#### Option 模式（函数选项模式）

Agent、OpenAI Client 等可配置对象均采用 Option 模式构造：

```go
agent, err := agent.NewAgent(
    agent.WithModel(client),           // 注入大模型（唯一必填）
    agent.WithTools(shellTool),        // 注册工具
    agent.WithMaxIterations(20),       // 安全阀
    agent.WithEventSink(sink),         // 注入事件订阅者
)
```

**优势**：可选参数友好、向前兼容（新增字段不破坏调用方）、惰性求值。

#### 事件驱动架构

Agent 内部的状态变化通过 **EventSink** 接口发出结构化事件，UI 层自主决定如何消费。同一份事件，TUI 可以渲染成气泡，REPL 可以打印成日志——两者完全解耦。

#### 依赖注入

核心依赖全部通过接口注入：

- **`agent.Model`** 接口 → 可替换为 Anthropic、Gemini 等任意模型
- **`agent.Tool`** 接口 → 可新增 FileTool、HTTPClientTool 等任意工具
- **`agent.EventSink`** 接口 → TUI 注入 channelSink，REPL 注入 stderrSink

这就是 **"依赖倒置原则（DIP）"** 的落地：高层模块定义抽象，低层模块提供实现。

---

## 7. 开发指南

### 构建命令

```bash
# 开发构建
go build ./cmd/tinycode

# 带版本信息的正式构建
go build -ldflags "-X tinycode/internal/cli.Version=1.0.0 -X tinycode/internal/cli.Commit=abc123" ./cmd/tinycode
```

### 运行测试

```bash
# 运行全部测试
go test ./...

# 带覆盖率报告
go test -cover ./...

# 单包测试示例
go test ./internal/agent/...
go test ./internal/tools/shell/...
```

### 代码检查

```bash
# 格式化代码
go fmt ./...

# 静态检查
go vet ./...

# 整理依赖
go mod tidy
```

---

## 8. 配置优先级

tinyCode 的配置采用**四层优先级**机制，从高到低依次为：

**CLI Flag > 环境变量 > 配置文件 `config.yaml` > 默认值**

| 优先级 | 来源 | 示例 |
|--------|------|------|
| 1（最高） | CLI 显式参数 | `--model gpt-4o` |
| 2 | 环境变量 | `OPENAI_MODEL=gpt-4o-mini` |
| 3 | 配置文件 | `model: gpt-4o-mini` |
| 4（最低） | 内置默认值 | `gpt-4o-mini` |

**实际场景举例：**

- 你运行 `tinycode --model gpt-4o` → 使用 `gpt-4o`（flag 最高优先级）
- 没传 flag，但设置了 `OPENAI_MODEL=gpt-4o-mini` → 使用 `gpt-4o-mini`
- flag 与环境变量都没有，`config.yaml` 里写了 `model: gpt-4o-mini` → 使用配置文件值
- 以上皆无 → 使用默认值 `gpt-4o-mini`

> `api_key` / `base_url` / `model` / `trace` / `trace_dir` 这五项同时支持配置文件；其他字段（如 `work_dir`）请通过 flag 或环境变量传入。

这种设计的好处是：**日常通过 `config.yaml` 或环境变量固定全局配置，临时需求通过 flag 覆盖**，灵活且不易出错。

---

## 9. License

MIT License

---

> 如果你在使用过程中遇到问题，欢迎提交 Issue 或 PR。tinyCode 的设计哲学是 **"好的架构不是能做什么，而是能不做什么"** —— 希望这份克制能让代码更易读懂、更易扩展。
