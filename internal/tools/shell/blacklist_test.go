package shell

import (
	"testing"
)

func TestDefaultBlacklist_NotEmpty(t *testing.T) {
	bl := defaultBlacklist()
	if len(bl) == 0 {
		t.Fatal("expected default blacklist to be non-empty")
	}
}

func TestIsBlocked_BlacklistedCommand_ReturnsBlocked(t *testing.T) {
	tool := New("")
	tests := []string{
		"rm -rf /",
		"rm -rf /*",
		"mkfs",
		"git push --force",
		"shutdown",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			reason, blocked := tool.isBlocked(cmd)
			if !blocked {
				t.Fatalf("expected command %q to be blocked", cmd)
			}
			if reason == "" {
				t.Fatal("expected non-empty block reason")
			}
		})
	}
}

func TestIsBlocked_SafeCommand_NotBlocked(t *testing.T) {
	tool := New("")
	tests := []string{
		"echo hello",
		"ls -la",
		"git status",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			reason, blocked := tool.isBlocked(cmd)
			if blocked {
				t.Fatalf("expected command %q not to be blocked, got reason: %s", cmd, reason)
			}
		})
	}
}
