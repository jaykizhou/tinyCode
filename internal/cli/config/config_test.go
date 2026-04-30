package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tinycode/internal/cli/config"

	"github.com/spf13/pflag"
)

// clearEnv 把受配置影响的环境变量置空，确保测试行为与宿主机隔离。
func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv(config.EnvAPIKey, "")
	t.Setenv(config.EnvBaseURL, "")
	t.Setenv(config.EnvModel, "")
}

func TestFinalize_APIKeyEmpty_ReturnsError(t *testing.T) {
	clearEnv(t)
	cfg := &config.RuntimeConfig{}
	err := cfg.Finalize(nil)
	if err == nil {
		t.Fatal("expected error when APIKey is empty, got nil")
	}
	if !strings.Contains(err.Error(), "API Key") {
		t.Fatalf("expected error message to contain 'API Key', got: %v", err)
	}
}

func TestFinalize_APIKeyNonEmpty_Success(t *testing.T) {
	clearEnv(t)
	cfg := &config.RuntimeConfig{APIKey: "sk-test"}
	err := cfg.Finalize(nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestFinalize_WorkDirEmpty_FillsCurrentDir(t *testing.T) {
	clearEnv(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	cfg := &config.RuntimeConfig{APIKey: "sk-test"}
	err = cfg.Finalize(nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.WorkDir != wd {
		t.Fatalf("expected WorkDir to be %q, got %q", wd, cfg.WorkDir)
	}
}

func TestFinalize_MaxIterationsZero_ResetsToDefault(t *testing.T) {
	clearEnv(t)
	cfg := &config.RuntimeConfig{APIKey: "sk-test", MaxIterations: 0}
	err := cfg.Finalize(nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.MaxIterations != config.DefaultMaxIterations {
		t.Fatalf("expected MaxIterations to be %d, got %d", config.DefaultMaxIterations, cfg.MaxIterations)
	}
}

func TestFinalize_MaxIterationsNegative_ResetsToDefault(t *testing.T) {
	clearEnv(t)
	cfg := &config.RuntimeConfig{APIKey: "sk-test", MaxIterations: -5}
	err := cfg.Finalize(nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.MaxIterations != config.DefaultMaxIterations {
		t.Fatalf("expected MaxIterations to be %d, got %d", config.DefaultMaxIterations, cfg.MaxIterations)
	}
}

func TestFinalize_MaxIterationsPositive_KeepsValue(t *testing.T) {
	clearEnv(t)
	cfg := &config.RuntimeConfig{APIKey: "sk-test", MaxIterations: 10}
	err := cfg.Finalize(nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.MaxIterations != 10 {
		t.Fatalf("expected MaxIterations to be 10, got %d", cfg.MaxIterations)
	}
}

// ========== 配置文件相关测试 ==========

// writeConfig 在 tmpDir 中写入 config.yaml 并返回其绝对路径。
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp config failed: %v", err)
	}
	return path
}

func TestFinalize_FileConfig_FillsMissingFields(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, `
api_key: sk-from-file
base_url: https://file.example.com/v1
model: file-model
`)
	cfg := &config.RuntimeConfig{
		BaseURL:    config.DefaultBaseURL,
		Model:      config.DefaultModel,
		ConfigPath: path,
	}
	if err := cfg.Finalize(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "sk-from-file" {
		t.Fatalf("APIKey: want sk-from-file, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://file.example.com/v1" {
		t.Fatalf("BaseURL: want file value, got %q", cfg.BaseURL)
	}
	if cfg.Model != "file-model" {
		t.Fatalf("Model: want file-model, got %q", cfg.Model)
	}
}

func TestFinalize_EnvOverridesFile(t *testing.T) {
	clearEnv(t)
	t.Setenv(config.EnvAPIKey, "sk-env")
	t.Setenv(config.EnvModel, "env-model")
	path := writeConfig(t, `
api_key: sk-file
model: file-model
base_url: https://file.example.com/v1
`)
	cfg := &config.RuntimeConfig{
		BaseURL:    config.DefaultBaseURL,
		Model:      config.DefaultModel,
		ConfigPath: path,
	}
	if err := cfg.Finalize(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "sk-env" {
		t.Fatalf("APIKey: env should win, got %q", cfg.APIKey)
	}
	if cfg.Model != "env-model" {
		t.Fatalf("Model: env should win, got %q", cfg.Model)
	}
	// 未设置 env 的 base_url 仍由文件填充
	if cfg.BaseURL != "https://file.example.com/v1" {
		t.Fatalf("BaseURL: file should win when env absent, got %q", cfg.BaseURL)
	}
}

func TestFinalize_FlagOverridesEnvAndFile(t *testing.T) {
	clearEnv(t)
	t.Setenv(config.EnvAPIKey, "sk-env")
	t.Setenv(config.EnvModel, "env-model")
	path := writeConfig(t, `
api_key: sk-file
model: file-model
`)
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg := &config.RuntimeConfig{}
	config.BindFlags(fs, cfg)

	// 模拟用户在命令行传入：--api-key sk-flag --model flag-model --config <path>
	if err := fs.Parse([]string{
		"--api-key", "sk-flag",
		"--model", "flag-model",
		"--config", path,
	}); err != nil {
		t.Fatalf("parse flags failed: %v", err)
	}
	if err := cfg.Finalize(fs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "sk-flag" {
		t.Fatalf("APIKey: flag should win, got %q", cfg.APIKey)
	}
	if cfg.Model != "flag-model" {
		t.Fatalf("Model: flag should win, got %q", cfg.Model)
	}
}

func TestFinalize_MissingFileIsIgnored(t *testing.T) {
	clearEnv(t)
	cfg := &config.RuntimeConfig{
		APIKey:     "sk-test",
		ConfigPath: filepath.Join(t.TempDir(), "does-not-exist.yaml"),
	}
	if err := cfg.Finalize(nil); err != nil {
		t.Fatalf("missing file should be ignored, got: %v", err)
	}
}

func TestFinalize_MalformedFileReturnsError(t *testing.T) {
	clearEnv(t)
	path := writeConfig(t, "this is not a yaml line without colon\n")
	cfg := &config.RuntimeConfig{ConfigPath: path}
	err := cfg.Finalize(nil)
	if err == nil {
		t.Fatal("expected error on malformed yaml, got nil")
	}
	if !strings.Contains(err.Error(), "配置文件") {
		t.Fatalf("expected error mention '配置文件', got: %v", err)
	}
}

// ========== flag 绑定测试 ==========

func TestBindFlags_ContainsExpectedFlagNames(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg := &config.RuntimeConfig{}
	config.BindFlags(fs, cfg)

	expectedFlags := []string{"base-url", "model", "api-key", "work-dir", "max-iter", "system", "verbose", "config"}
	for _, name := range expectedFlags {
		if fs.Lookup(name) == nil {
			t.Fatalf("expected flag %q to be registered", name)
		}
	}
}

func TestBindFlags_DefaultValuesMatchConstants(t *testing.T) {
	// 清除环境变量，确保默认值不受外部环境影响
	clearEnv(t)

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg := &config.RuntimeConfig{}
	config.BindFlags(fs, cfg)

	cases := []struct {
		name     string
		expected string
	}{
		{"base-url", config.DefaultBaseURL},
		{"model", config.DefaultModel},
		{"config", config.DefaultConfigPath},
	}
	for _, c := range cases {
		f := fs.Lookup(c.name)
		if f == nil {
			t.Fatalf("flag %q not found", c.name)
		}
		if f.DefValue != c.expected {
			t.Fatalf("expected default value of %q to be %q, got %q", c.name, c.expected, f.DefValue)
		}
	}

	maxIterFlag := fs.Lookup("max-iter")
	if maxIterFlag == nil {
		t.Fatal("flag 'max-iter' not found")
	}
	if maxIterFlag.DefValue != "25" {
		t.Fatalf("expected default value of 'max-iter' to be '25', got %q", maxIterFlag.DefValue)
	}

	verboseFlag := fs.Lookup("verbose")
	if verboseFlag == nil {
		t.Fatal("flag 'verbose' not found")
	}
	if verboseFlag.DefValue != "false" {
		t.Fatalf("expected default value of 'verbose' to be 'false', got %q", verboseFlag.DefValue)
	}

	apiKeyFlag := fs.Lookup("api-key")
	if apiKeyFlag == nil {
		t.Fatal("flag 'api-key' not found")
	}
	// api-key 的默认值应为空字符串：环境变量与文件值在 Finalize 阶段再合并
	if apiKeyFlag.DefValue != "" {
		t.Fatalf("expected default value of 'api-key' to be empty, got %q", apiKeyFlag.DefValue)
	}
}
