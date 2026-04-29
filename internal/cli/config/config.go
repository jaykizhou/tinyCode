// Package config 负责解析 CLI flag、环境变量与默认值，
// 产出供 bootstrap / UI 使用的 RuntimeConfig。
//
// 合并优先级：CLI flag > 环境变量 > 默认值。
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// 环境变量键名。与 OpenAI 官方约定对齐。
const (
	EnvAPIKey  = "OPENAI_API_KEY"
	EnvBaseURL = "OPENAI_BASE_URL"
	EnvModel   = "OPENAI_MODEL"
)

// 默认值常量。
const (
	DefaultBaseURL       = "https://api.openai.com/v1"
	DefaultModel         = "gpt-4o-mini"
	DefaultMaxIterations = 25
)

// RuntimeConfig 是运行一次 tinycode 所需的全部配置快照。
//
// 该结构体只描述"运行时配置"，不含任何业务对象（Agent、Model、Tool）。
// 业务对象由 bootstrap 包根据 RuntimeConfig 装配。
type RuntimeConfig struct {
	APIKey        string // OpenAI API Key，必填
	BaseURL       string // OpenAI 兼容端点
	Model         string // 模型名
	WorkDir       string // Shell 工具工作目录
	MaxIterations int    // Agent 主循环上限
	SystemPrompt  string // 覆盖默认系统提示；为空时沿用 agent 内置
	Verbose       bool   // 是否输出详细事件日志
}

// BindFlags 在给定 FlagSet 上注册所有 CLI flag，同时把环境变量作为默认值。
//
// 这样可以做到：
//   - 用户直接 `tinycode` → 使用环境变量或默认值；
//   - 用户 `tinycode --model gpt-4o` → CLI flag 覆盖环境变量；
//   - 所有子命令共享同一份 PersistentFlags，避免重复定义。
func BindFlags(fs *pflag.FlagSet, cfg *RuntimeConfig) {
	fs.StringVar(&cfg.BaseURL, "base-url",
		firstNonEmpty(os.Getenv(EnvBaseURL), DefaultBaseURL),
		"OpenAI 兼容 API 的 base URL")
	fs.StringVar(&cfg.Model, "model",
		firstNonEmpty(os.Getenv(EnvModel), DefaultModel),
		"模型名称，例如 gpt-4o-mini")
	fs.StringVar(&cfg.APIKey, "api-key",
		os.Getenv(EnvAPIKey),
		"API Key；不填则读取环境变量 "+EnvAPIKey)
	fs.StringVar(&cfg.WorkDir, "work-dir", "",
		"Shell 工具的工作目录，默认为当前目录")
	fs.IntVar(&cfg.MaxIterations, "max-iter",
		DefaultMaxIterations,
		"Agent 主循环最大迭代次数")
	fs.StringVar(&cfg.SystemPrompt, "system", "",
		"覆盖默认系统提示词；为空时使用内置提示")
	fs.BoolVarP(&cfg.Verbose, "verbose", "v", false,
		"输出工具调用等详细日志")
}

// Finalize 在子命令 RunE 开始时调用，做最后一步校验与补全。
//
// 它负责：
//  1. 校验必要字段（APIKey）；
//  2. 在 WorkDir 为空时填入当前工作目录；
//  3. 对 MaxIterations 做保底。
func (c *RuntimeConfig) Finalize() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("缺少 API Key，请设置环境变量 %s 或使用 --api-key", EnvAPIKey)
	}
	if c.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("获取工作目录失败: %w", err)
		}
		c.WorkDir = wd
	}
	if c.MaxIterations <= 0 {
		c.MaxIterations = DefaultMaxIterations
	}
	return nil
}

// firstNonEmpty 返回参数中第一个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
