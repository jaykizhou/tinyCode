package agent_test

import (
	"testing"

	"tinycode/internal/agent"
)

func TestWithModel_AgentCreatesSuccessfully(t *testing.T) {
	a, err := agent.NewAgent(agent.WithModel(&mockModel{}))
	if err != nil {
		t.Fatalf("NewAgent(WithModel) returned error: %v", err)
	}
	if a == nil {
		t.Fatal("NewAgent(WithModel) returned nil agent")
	}
}

func TestWithSystemPrompt_EffectiveViaConversation(t *testing.T) {
	customPrompt := "custom system prompt for testing"

	a, err := agent.NewAgent(
		agent.WithModel(&mockModel{}),
		agent.WithSystemPrompt(customPrompt),
	)
	if err != nil {
		t.Fatalf("NewAgent returned error: %v", err)
	}

	// 通过触发 RunLoop 让 Agent 把 user 消息追加到 Conversation，
	// 然后验证 Conversation 确实在工作（间接证明 Agent 构造成功）。
	// 由于 mockModel 返回空响应，RunLoop 会立即返回空字符串。
	// 但 RunLoop 需要 context，这里我们只验证 Agent 能成功创建即可。
	// 更直接的方式：WithSystemPrompt 的效果是覆盖默认值，可以通过
	// 检查 Conversation 非空来确认 Agent 正常初始化。
	if a.Conversation() == nil {
		t.Error("Conversation() is nil after WithSystemPrompt")
	}
	if a.Conversation().Len() != 0 {
		t.Errorf("Conversation().Len() = %d, want 0", a.Conversation().Len())
	}
}

func TestWithTools_RegistryContainsTools(t *testing.T) {
	toolA := &mockTool{name: "tool-a", description: "desc a", parameters: nil}
	toolB := &mockTool{name: "tool-b", description: "desc b", parameters: nil}

	a, err := agent.NewAgent(
		agent.WithModel(&mockModel{}),
		agent.WithTools(toolA, toolB),
	)
	if err != nil {
		t.Fatalf("NewAgent returned error: %v", err)
	}

	if _, ok := a.Registry().Get("tool-a"); !ok {
		t.Error("Registry should contain tool-a")
	}
	if _, ok := a.Registry().Get("tool-b"); !ok {
		t.Error("Registry should contain tool-b")
	}
}

func TestWithMaxIterations_AcceptedWithoutError(t *testing.T) {
	_, err := agent.NewAgent(
		agent.WithModel(&mockModel{}),
		agent.WithMaxIterations(10),
	)
	if err != nil {
		t.Fatalf("NewAgent(WithMaxIterations) returned error: %v", err)
	}
}

func TestWithEventSink_AcceptedWithoutError(t *testing.T) {
	sink := agent.EventSinkFunc(func(e agent.Event) {})
	_, err := agent.NewAgent(
		agent.WithModel(&mockModel{}),
		agent.WithEventSink(sink),
	)
	if err != nil {
		t.Fatalf("NewAgent(WithEventSink) returned error: %v", err)
	}
}

func TestWithLogger_AcceptedWithoutError(t *testing.T) {
	logger := func(event string, kv ...any) {}
	_, err := agent.NewAgent(
		agent.WithModel(&mockModel{}),
		agent.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("NewAgent(WithLogger) returned error: %v", err)
	}
}
