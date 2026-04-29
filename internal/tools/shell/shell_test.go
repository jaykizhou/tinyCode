package shell_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"tinycode/internal/tools/shell"
)

func TestNew_ToolNotNil(t *testing.T) {
	tool := shell.New("/tmp")
	if tool == nil {
		t.Fatal("expected non-nil Tool, got nil")
	}
}

func TestName_ReturnsShell(t *testing.T) {
	tool := shell.New("")
	if tool.Name() != "shell" {
		t.Fatalf("expected Name() to return 'shell', got %q", tool.Name())
	}
}

func TestDescription_NotEmpty(t *testing.T) {
	tool := shell.New("")
	if tool.Description() == "" {
		t.Fatal("expected Description() to return non-empty string")
	}
}

func TestParameters_ValidJSON(t *testing.T) {
	tool := shell.New("")
	raw := tool.Parameters()
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("expected valid JSON, got unmarshal error: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("expected schema type to be 'object', got %v", schema["type"])
	}
}

func TestExecute_InvalidJSON_ReturnsError(t *testing.T) {
	tool := shell.New("")
	ctx := context.Background()
	_, err := tool.Execute(ctx, json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "参数反序列化失败") {
		t.Fatalf("expected error to contain '参数反序列化失败', got: %v", err)
	}
}

func TestExecute_EmptyCommand_ReturnsError(t *testing.T) {
	tool := shell.New("")
	ctx := context.Background()
	_, err := tool.Execute(ctx, json.RawMessage(`{"command":""}`))
	if err == nil {
		t.Fatal("expected error for empty command, got nil")
	}
	if !strings.Contains(err.Error(), "参数 command 不能为空") {
		t.Fatalf("expected error to contain '参数 command 不能为空', got: %v", err)
	}
}

func TestExecute_NormalCommand_OutputContainsExpected(t *testing.T) {
	tool := shell.New("")
	ctx := context.Background()
	result, err := tool.Execute(ctx, json.RawMessage(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Fatalf("expected output to contain 'hello', got: %q", result)
	}
}

func TestExecute_BlacklistedCommand_ReturnsBlockMessage(t *testing.T) {
	tool := shell.New("")
	ctx := context.Background()
	result, err := tool.Execute(ctx, json.RawMessage(`{"command":"rm -rf /"}`))
	if err != nil {
		t.Fatalf("expected no error (blocked commands return message, not error), got: %v", err)
	}
	if !strings.Contains(result, "命令被安全策略拦截") {
		t.Fatalf("expected result to contain '命令被安全策略拦截', got: %q", result)
	}
}
