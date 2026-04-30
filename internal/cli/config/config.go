// Package config 负责解析 CLI flag、环境变量、配置文件与默认值，
// 产出供 bootstrap / UI 使用的 RuntimeConfig。
//
// 合并优先级：CLI flag > 环境变量 > 配置文件 > 默认值。
package config

import (
	"errors"
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
	// DefaultConfigPath 默认在当前工作目录查找；找不到即忽略，不报错。
	DefaultConfigPath = "config.yaml"
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
	ConfigPath    string // YAML 配置文件路径；不存在则忽略
}

// BindFlags 在给定 FlagSet 上注册所有 CLI flag。
//
// 与旧实现不同：flag 的默认值不再"吞掉"环境变量与配置文件，而是统一留到
// Finalize 阶段按优先级合并，避免 `pflag.Changed()` 无法区分"环境变量填充"
// 与"用户显式 flag"两种情况。
func BindFlags(fs *pflag.FlagSet, cfg *RuntimeConfig) {
	fs.StringVar(&cfg.BaseURL, "base-url", DefaultBaseURL,
		"OpenAI 兼容 API 的 base URL")
	fs.StringVar(&cfg.Model, "model", DefaultModel,
		"模型名称，例如 gpt-4o-mini")
	fs.StringVar(&cfg.APIKey, "api-key", "",
		"API Key；不填则依次读取环境变量 "+EnvAPIKey+" / 配置文件 api_key")
	fs.StringVar(&cfg.WorkDir, "work-dir", "",
		"Shell 工具的工作目录，默认为当前目录")
	fs.IntVar(&cfg.MaxIterations, "max-iter", DefaultMaxIterations,
		"Agent 主循环最大迭代次数")
	fs.StringVar(&cfg.SystemPrompt, "system", "",
		"覆盖默认系统提示词；为空时使用内置提示")
	fs.BoolVarP(&cfg.Verbose, "verbose", "v", false,
		"输出工具调用等详细日志")
	fs.StringVar(&cfg.ConfigPath, "config", DefaultConfigPath,
		"YAML 配置文件路径；文件不存在将被忽略")
}

// Finalize 在子命令 RunE 开始时调用，做最后一步合并与校验。
//
// 合并顺序（优先级高 → 低）：CLI flag > 环境变量 > 配置文件 > 默认值。
// 传入的 fs 用于判断用户是否在命令行显式指定了某个 flag；
// 当 fs 为 nil 时（如单元测试直接构造 RuntimeConfig），
// 视为"没有任何显式 flag"，仍可完成后续三层合并。
func (c *RuntimeConfig) Finalize(fs *pflag.FlagSet) error {
	file, err := LoadFile(c.ConfigPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	applyStringOverride(fs, "api-key", &c.APIKey, os.Getenv(EnvAPIKey), file.APIKey)
	applyStringOverride(fs, "base-url", &c.BaseURL, os.Getenv(EnvBaseURL), file.BaseURL)
	applyStringOverride(fs, "model", &c.Model, os.Getenv(EnvModel), file.Model)

	// if strings.TrimSpace(c.APIKey) == "" {
	// 	return fmt.Errorf(
	// 		"缺少 API Key，请通过 --api-key、环境变量 %s 或配置文件 %s 提供",
	// 		EnvAPIKey, c.effectiveConfigPath())
	// }
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

// applyStringOverride 按优先级把值写入 target：
//  1. 若 fs 非 nil 且 flag 已被用户在命令行显式修改 → 保留 target（flag 最高优先级）；
//  2. 否则 env 非空 → env 覆盖 target；
//  3. 否则 fileVal 非空 → 文件值覆盖 target；
//  4. 否则保留 target 当前值（来自 BindFlags 的默认值或结构体零值）。
func applyStringOverride(fs *pflag.FlagSet, flagName string, target *string, envVal, fileVal string) {
	if fs != nil && fs.Changed(flagName) {
		return
	}
	if envVal != "" {
		*target = envVal
		return
	}
	if fileVal != "" {
		*target = fileVal
		return
	}
}

// effectiveConfigPath 返回给错误信息展示用的配置文件路径。
func (c *RuntimeConfig) effectiveConfigPath() string {
	if strings.TrimSpace(c.ConfigPath) == "" {
		return DefaultConfigPath
	}
	return c.ConfigPath
}
