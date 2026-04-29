package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"tinycode/internal/cli/config"
)

func TestFinalize_APIKeyEmpty_ReturnsError(t *testing.T) {
	cfg := &config.RuntimeConfig{}
	err := cfg.Finalize()
	if err == nil {
		t.Fatal("expected error when APIKey is empty, got nil")
	}
	if !strings.Contains(err.Error(), "API Key") {
		t.Fatalf("expected error message to contain 'API Key', got: %v", err)
	}
}

func TestFinalize_APIKeyNonEmpty_Success(t *testing.T) {
	cfg := &config.RuntimeConfig{APIKey: "sk-test"}
	err := cfg.Finalize()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestFinalize_WorkDirEmpty_FillsCurrentDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	cfg := &config.RuntimeConfig{APIKey: "sk-test"}
	err = cfg.Finalize()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.WorkDir != wd {
		t.Fatalf("expected WorkDir to be %q, got %q", wd, cfg.WorkDir)
	}
}

func TestFinalize_MaxIterationsZero_ResetsToDefault(t *testing.T) {
	cfg := &config.RuntimeConfig{APIKey: "sk-test", MaxIterations: 0}
	err := cfg.Finalize()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.MaxIterations != config.DefaultMaxIterations {
		t.Fatalf("expected MaxIterations to be %d, got %d", config.DefaultMaxIterations, cfg.MaxIterations)
	}
}

func TestFinalize_MaxIterationsNegative_ResetsToDefault(t *testing.T) {
	cfg := &config.RuntimeConfig{APIKey: "sk-test", MaxIterations: -5}
	err := cfg.Finalize()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.MaxIterations != config.DefaultMaxIterations {
		t.Fatalf("expected MaxIterations to be %d, got %d", config.DefaultMaxIterations, cfg.MaxIterations)
	}
}

func TestFinalize_MaxIterationsPositive_KeepsValue(t *testing.T) {
	cfg := &config.RuntimeConfig{APIKey: "sk-test", MaxIterations: 10}
	err := cfg.Finalize()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.MaxIterations != 10 {
		t.Fatalf("expected MaxIterations to be 10, got %d", cfg.MaxIterations)
	}
}

func TestBindFlags_ContainsExpectedFlagNames(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg := &config.RuntimeConfig{}
	config.BindFlags(fs, cfg)

	expectedFlags := []string{"base-url", "model", "api-key", "work-dir", "max-iter", "system", "verbose"}
	for _, name := range expectedFlags {
		if fs.Lookup(name) == nil {
			t.Fatalf("expected flag %q to be registered", name)
		}
	}
}

func TestBindFlags_DefaultValuesMatchConstants(t *testing.T) {
	// 清除环境变量，确保默认值不受外部环境影响
	t.Setenv(config.EnvAPIKey, "")
	t.Setenv(config.EnvBaseURL, "")
	t.Setenv(config.EnvModel, "")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg := &config.RuntimeConfig{}
	config.BindFlags(fs, cfg)

	cases := []struct {
		name     string
		expected string
	}{
		{"base-url", config.DefaultBaseURL},
		{"model", config.DefaultModel},
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
}
