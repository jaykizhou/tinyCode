package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"tinycode/internal/agent"
)

// mockTool 是一个简单的 Tool 接口实现，用于测试。
type mockTool struct {
	name        string
	description string
	parameters  json.RawMessage
}

func (m *mockTool) Name() string { return m.name }

func (m *mockTool) Description() string { return m.description }

func (m *mockTool) Parameters() json.RawMessage { return m.parameters }

func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	return "mock-result", nil
}

func TestRegistry_NewRegistry_IsEmpty(t *testing.T) {
	reg := agent.NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	_, ok := reg.Get("any")
	if ok {
		t.Error("new registry should not contain any tool")
	}

	defs := reg.Definitions()
	if len(defs) != 0 {
		t.Errorf("Definitions() length = %d, want 0", len(defs))
	}
}

func TestRegistry_Register_And_Get(t *testing.T) {
	reg := agent.NewRegistry()
	tool := &mockTool{
		name:        "test-tool",
		description: "a test tool",
		parameters:  json.RawMessage(`{"type":"object"}`),
	}

	reg.Register(tool)

	got, ok := reg.Get("test-tool")
	if !ok {
		t.Fatal("Get(test-tool) returned ok=false, want true")
	}
	if got.Name() != "test-tool" {
		t.Errorf("Get().Name() = %q, want %q", got.Name(), "test-tool")
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	reg := agent.NewRegistry()

	_, ok := reg.Get("missing")
	if ok {
		t.Error("Get(missing) returned ok=true, want false")
	}
}

func TestRegistry_Definitions_OrderIsStable(t *testing.T) {
	reg := agent.NewRegistry()

	toolA := &mockTool{name: "tool-a", description: "desc a", parameters: json.RawMessage(`{}`)}
	toolB := &mockTool{name: "tool-b", description: "desc b", parameters: json.RawMessage(`{}`)}
	toolC := &mockTool{name: "tool-c", description: "desc c", parameters: json.RawMessage(`{}`)}

	reg.Register(toolA)
	reg.Register(toolB)
	reg.Register(toolC)

	defs := reg.Definitions()
	if len(defs) != 3 {
		t.Fatalf("Definitions() length = %d, want 3", len(defs))
	}

	expected := []string{"tool-a", "tool-b", "tool-c"}
	for i, exp := range expected {
		if defs[i].Name != exp {
			t.Errorf("Definitions()[%d].Name = %q, want %q", i, defs[i].Name, exp)
		}
	}

	// 验证 Description 和 Parameters 也正确
	if defs[0].Description != "desc a" {
		t.Errorf("Definitions()[0].Description = %q, want %q", defs[0].Description, "desc a")
	}
	if string(defs[0].Parameters) != "{}" {
		t.Errorf("Definitions()[0].Parameters = %s, want %s", defs[0].Parameters, "{}")
	}
}

func TestRegistry_Register_OverwriteKeepsOrder(t *testing.T) {
	reg := agent.NewRegistry()

	toolA := &mockTool{name: "tool-a", description: "first", parameters: json.RawMessage(`{}`)}
	toolB := &mockTool{name: "tool-b", description: "first b", parameters: json.RawMessage(`{}`)}

	reg.Register(toolA)
	reg.Register(toolB)

	// 覆盖 tool-a，顺序应保持不变
	toolA2 := &mockTool{name: "tool-a", description: "second", parameters: json.RawMessage(`{}`)}
	reg.Register(toolA2)

	defs := reg.Definitions()
	if len(defs) != 2 {
		t.Fatalf("Definitions() length = %d, want 2", len(defs))
	}

	if defs[0].Name != "tool-a" {
		t.Errorf("Definitions()[0].Name = %q, want %q", defs[0].Name, "tool-a")
	}
	if defs[0].Description != "second" {
		t.Errorf("Definitions()[0].Description = %q, want %q", defs[0].Description, "second")
	}
	if defs[1].Name != "tool-b" {
		t.Errorf("Definitions()[1].Name = %q, want %q", defs[1].Name, "tool-b")
	}
}
