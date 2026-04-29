package agent_test

import (
	"context"
	"errors"
	"testing"

	"tinycode/internal/agent"
)

// mockModel 是一个简单的 Model 接口实现，用于测试。
type mockModel struct{}

func (m *mockModel) Name() string { return "mock-model" }

func (m *mockModel) Complete(ctx context.Context, req agent.CompletionRequest) (agent.CompletionResponse, error) {
	return agent.CompletionResponse{}, nil
}

func TestNewAgent_WithoutModel_ReturnsErrModelRequired(t *testing.T) {
	_, err := agent.NewAgent()
	if err == nil {
		t.Fatal("NewAgent() without model returned nil error, want ErrModelRequired")
	}
	if !errors.Is(err, agent.ErrModelRequired) {
		t.Errorf("NewAgent() error = %v, want ErrModelRequired", err)
	}
}

func TestNewAgent_WithModel_Succeeds(t *testing.T) {
	a, err := agent.NewAgent(agent.WithModel(&mockModel{}))
	if err != nil {
		t.Fatalf("NewAgent(WithModel) returned error: %v", err)
	}
	if a == nil {
		t.Fatal("NewAgent(WithModel) returned nil agent")
	}
}

func TestNewAgent_RegistryAndConversationAreNonNil(t *testing.T) {
	a, err := agent.NewAgent(agent.WithModel(&mockModel{}))
	if err != nil {
		t.Fatalf("NewAgent returned error: %v", err)
	}

	if a.Registry() == nil {
		t.Error("Registry() is nil, want non-nil")
	}
	if a.Conversation() == nil {
		t.Error("Conversation() is nil, want non-nil")
	}
}

func TestNewAgent_WithTools_RegisteredInRegistry(t *testing.T) {
	tool := &mockTool{
		name:        "my-tool",
		description: "a test tool",
		parameters:  nil,
	}

	a, err := agent.NewAgent(
		agent.WithModel(&mockModel{}),
		agent.WithTools(tool),
	)
	if err != nil {
		t.Fatalf("NewAgent returned error: %v", err)
	}

	got, ok := a.Registry().Get("my-tool")
	if !ok {
		t.Fatal("Registry().Get(my-tool) returned ok=false, want true")
	}
	if got.Name() != "my-tool" {
		t.Errorf("Registry().Get().Name() = %q, want %q", got.Name(), "my-tool")
	}
}
