package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tinycode/internal/cli/config"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}
	return path
}

func TestLoadFile_EmptyPathReturnsEmpty(t *testing.T) {
	got, err := config.LoadFile("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != (config.FileConfig{}) {
		t.Fatalf("expected zero FileConfig for empty path, got %+v", got)
	}
}

func TestLoadFile_NonexistentReturnsEmpty(t *testing.T) {
	got, err := config.LoadFile(filepath.Join(t.TempDir(), "no-such.yaml"))
	if err != nil {
		t.Fatalf("nonexistent file should not error, got: %v", err)
	}
	if got != (config.FileConfig{}) {
		t.Fatalf("expected zero FileConfig, got %+v", got)
	}
}

func TestLoadFile_ParsesAllSupportedFields(t *testing.T) {
	path := writeTempFile(t, `
# 顶部注释
api_key: sk-abc-123
base_url: https://api.example.com/v1
model: gpt-4o

# 未知字段应被忽略
work_dir: /tmp
`)
	got, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.APIKey != "sk-abc-123" {
		t.Errorf("APIKey: want sk-abc-123, got %q", got.APIKey)
	}
	if got.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL: want https://api.example.com/v1, got %q", got.BaseURL)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("Model: want gpt-4o, got %q", got.Model)
	}
}

func TestLoadFile_HandlesQuotedValues(t *testing.T) {
	path := writeTempFile(t, `
api_key: "sk-in-double-quote"
base_url: 'https://api.example.com/v1'
model: "model-with-space "
`)
	got, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.APIKey != "sk-in-double-quote" {
		t.Errorf("APIKey: want sk-in-double-quote, got %q", got.APIKey)
	}
	if got.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL: want https://api.example.com/v1, got %q", got.BaseURL)
	}
	// 保留引号内部空格
	if got.Model != "model-with-space " {
		t.Errorf("Model: want 'model-with-space ', got %q", got.Model)
	}
}

func TestLoadFile_HandlesInlineComments(t *testing.T) {
	path := writeTempFile(t, `
api_key: sk-real # 行末注释会被去掉
model: gpt-4o    # 另一条
`)
	got, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.APIKey != "sk-real" {
		t.Errorf("APIKey: want sk-real, got %q", got.APIKey)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("Model: want gpt-4o, got %q", got.Model)
	}
}

func TestLoadFile_HashInsideValueKept(t *testing.T) {
	// 紧挨字符的 # 不是注释（前面没有空白），应当保留
	path := writeTempFile(t, `api_key: sk-abc#123`+"\n")
	got, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.APIKey != "sk-abc#123" {
		t.Errorf("APIKey: want sk-abc#123, got %q", got.APIKey)
	}
}

func TestLoadFile_MalformedLineReturnsError(t *testing.T) {
	path := writeTempFile(t, "this_is_not_yaml\n")
	_, err := config.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for malformed line, got nil")
	}
	if !strings.Contains(err.Error(), "解析") {
		t.Fatalf("expected error mention '解析', got: %v", err)
	}
}

func TestLoadFile_AliasKeysAccepted(t *testing.T) {
	// apiKey / baseUrl 等驼峰别名也应识别
	path := writeTempFile(t, `
apiKey: sk-alias
baseUrl: https://alias.example.com
`)
	got, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.APIKey != "sk-alias" {
		t.Errorf("APIKey: want sk-alias, got %q", got.APIKey)
	}
	if got.BaseURL != "https://alias.example.com" {
		t.Errorf("BaseURL: want https://alias.example.com, got %q", got.BaseURL)
	}
}

func TestLoadFile_EmptyFileReturnsEmpty(t *testing.T) {
	path := writeTempFile(t, "")
	got, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != (config.FileConfig{}) {
		t.Fatalf("expected zero FileConfig, got %+v", got)
	}
}
