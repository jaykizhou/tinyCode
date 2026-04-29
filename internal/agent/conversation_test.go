package agent_test

import (
	"testing"

	"tinycode/internal/agent"
)

func TestConversation_NewConversation_LenIsZero(t *testing.T) {
	conv := agent.NewConversation()
	if got := conv.Len(); got != 0 {
		t.Fatalf("NewConversation().Len() = %d, want 0", got)
	}
}

func TestConversation_Append_ReturnsIncrementingCount(t *testing.T) {
	conv := agent.NewConversation()

	msg1 := agent.Message{Role: agent.RoleUser, Content: "hello"}
	if got := conv.Append(msg1); got != 1 {
		t.Errorf("Append(msg1) = %d, want 1", got)
	}

	msg2 := agent.Message{Role: agent.RoleAssistant, Content: "hi"}
	if got := conv.Append(msg2); got != 2 {
		t.Errorf("Append(msg2) = %d, want 2", got)
	}

	msg3 := agent.Message{Role: agent.RoleUser, Content: "bye"}
	if got := conv.Append(msg3); got != 3 {
		t.Errorf("Append(msg3) = %d, want 3", got)
	}
}

func TestConversation_Snapshot_ReturnsCopy(t *testing.T) {
	conv := agent.NewConversation()
	conv.Append(agent.Message{Role: agent.RoleUser, Content: "original"})

	snap := conv.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("Snapshot() length = %d, want 1", len(snap))
	}
	if snap[0].Content != "original" {
		t.Errorf("Snapshot()[0].Content = %q, want %q", snap[0].Content, "original")
	}

	// 修改副本不应影响原始数据
	snap[0].Content = "modified"
	snap = conv.Snapshot()
	if snap[0].Content != "original" {
		t.Errorf("after modifying copy, Snapshot()[0].Content = %q, want %q", snap[0].Content, "original")
	}
}

func TestConversation_Len_AfterAppend(t *testing.T) {
	conv := agent.NewConversation()
	if conv.Len() != 0 {
		t.Fatalf("initial Len() = %d, want 0", conv.Len())
	}

	conv.Append(agent.Message{Role: agent.RoleUser, Content: "a"})
	if conv.Len() != 1 {
		t.Errorf("after 1 append, Len() = %d, want 1", conv.Len())
	}

	conv.Append(agent.Message{Role: agent.RoleUser, Content: "b"})
	if conv.Len() != 2 {
		t.Errorf("after 2 appends, Len() = %d, want 2", conv.Len())
	}
}
