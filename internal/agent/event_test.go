package agent_test

import (
	"encoding/json"
	"testing"

	"tinycode/internal/agent"
)

func TestEventSinkFunc_AdapterWorks(t *testing.T) {
	var received agent.Event
	var called bool

	fn := agent.EventSinkFunc(func(e agent.Event) {
		received = e
		called = true
	})

	event := agent.Event{
		Kind:    agent.EventIterStart,
		Iter:    1,
		Payload: "test-payload",
	}
	fn.Emit(event)

	if !called {
		t.Fatal("EventSinkFunc was not called")
	}
	if received.Kind != agent.EventIterStart {
		t.Errorf("received.Kind = %q, want %q", received.Kind, agent.EventIterStart)
	}
	if received.Iter != 1 {
		t.Errorf("received.Iter = %d, want 1", received.Iter)
	}
	if received.Payload != "test-payload" {
		t.Errorf("received.Payload = %q, want %q", received.Payload, "test-payload")
	}
}

func TestEvent_FieldCombination(t *testing.T) {
	event := agent.Event{
		Kind:       agent.EventToolCall,
		Iter:       3,
		ToolName:   "my-tool",
		ToolCallID: "call_123",
		Payload:    "result-data",
		Args:       json.RawMessage(`{"key":"value"}`),
	}

	if event.Kind != agent.EventToolCall {
		t.Errorf("Kind = %q, want %q", event.Kind, agent.EventToolCall)
	}
	if event.Iter != 3 {
		t.Errorf("Iter = %d, want 3", event.Iter)
	}
	if event.ToolName != "my-tool" {
		t.Errorf("ToolName = %q, want %q", event.ToolName, "my-tool")
	}
	if event.ToolCallID != "call_123" {
		t.Errorf("ToolCallID = %q, want %q", event.ToolCallID, "call_123")
	}
	if event.Payload != "result-data" {
		t.Errorf("Payload = %q, want %q", event.Payload, "result-data")
	}
	if string(event.Args) != `{"key":"value"}` {
		t.Errorf("Args = %s, want %s", event.Args, `{"key":"value"}`)
	}
}
