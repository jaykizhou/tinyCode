package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tinycode/internal/agent"
	"tinycode/internal/model/openai"
)

func TestNewClient(t *testing.T) {
	c := openai.NewClient("http://localhost/v1", "key", "gpt-4o")
	if c == nil {
		t.Fatal("expected non-nil Client")
	}
}

func TestClientName(t *testing.T) {
	c := openai.NewClient("http://localhost/v1", "key", "gpt-4o-mini")
	if got := c.Name(); got != "gpt-4o-mini" {
		t.Errorf("Name() = %q, want %q", got, "gpt-4o-mini")
	}
}

func TestWithHTTPClient(t *testing.T) {
	hc := &http.Client{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("WithHTTPClient panicked: %v", r)
		}
	}()
	_ = openai.NewClient("http://localhost/v1", "key", "model", openai.WithHTTPClient(hc))
}

func TestWithTemperature(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("WithTemperature panicked: %v", r)
		}
	}()
	_ = openai.NewClient("http://localhost/v1", "key", "model", openai.WithTemperature(0.7))
}

func TestClientComplete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("expected Authorization header Bearer test-key, got %s", auth)
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from mock",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := openai.NewClient(server.URL, "test-key", "gpt-4o", openai.WithHTTPClient(server.Client()))

	req := agent.CompletionRequest{
		SystemPrompt: "You are helpful",
		Messages: []agent.Message{
			{Role: agent.RoleUser, Content: "Hi"},
		},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if resp.Message.Content != "Hello from mock" {
		t.Errorf("Content = %q, want %q", resp.Message.Content, "Hello from mock")
	}
	if resp.Message.Role != agent.RoleAssistant {
		t.Errorf("Role = %q, want %q", resp.Message.Role, agent.RoleAssistant)
	}
	if resp.Stop != "stop" {
		t.Errorf("Stop = %q, want %q", resp.Stop, "stop")
	}
}

func TestClientComplete_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := openai.NewClient(server.URL, "key", "model", openai.WithHTTPClient(server.Client()))

	req := agent.CompletionRequest{}
	_, err := client.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}
