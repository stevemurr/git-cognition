package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stevemurr/git-cognition/internal/storage"
)

func testSession() *storage.Session {
	return &storage.Session{
		SessionID: "test-session",
		Task:      storage.Task{Prompt: "Add rate limiting to the auth endpoint"},
		Commits: []storage.Commit{
			{SHA: "abc1234", Message: "feat: add rate limiter", FilesChanged: []string{"auth/middleware.go", "auth/middleware_test.go"}},
		},
		ToolCalls: []storage.ToolCall{
			{Sequence: 1, Tool: "Read", Input: json.RawMessage(`{"file_path":"auth/middleware.go"}`)},
			{Sequence: 2, Tool: "Edit", Input: json.RawMessage(`{"file_path":"auth/middleware.go"}`)},
			{Sequence: 3, Tool: "Bash", Input: json.RawMessage(`{"command":"go test ./auth/..."}`)},
		},
		Reasoning: storage.Reasoning{
			FinalMessage: "I added a token bucket rate limiter. I considered a sliding window but it required Lua scripts.",
			Source:       "claude_final_message",
		},
	}
}

func TestBuildPrompt(t *testing.T) {
	session := testSession()
	prompt := buildPrompt(session)

	checks := []string{
		"## Task",
		"Add rate limiting",
		"## Commits",
		"abc1234",
		"auth/middleware.go",
		"## Tool Sequence",
		"1. Read auth/middleware.go",
		"2. Edit auth/middleware.go",
		"3. Bash: go test ./auth/...",
		"## Final Message",
		"token bucket rate limiter",
		"Extract the reasoning",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing %q", check)
		}
	}
}

func TestBuildPromptTruncation(t *testing.T) {
	session := testSession()
	session.Reasoning.FinalMessage = strings.Repeat("x", 4000)
	prompt := buildPrompt(session)

	if !strings.Contains(prompt, "[truncated]") {
		t.Error("expected truncation marker for long final message")
	}
}

func TestBuildPromptToolCallCap(t *testing.T) {
	session := testSession()
	session.ToolCalls = nil
	for i := 0; i < 40; i++ {
		session.ToolCalls = append(session.ToolCalls, storage.ToolCall{
			Sequence: i + 1,
			Tool:     "Read",
			Input:    json.RawMessage(`{"file_path":"file.go"}`),
		})
	}
	prompt := buildPrompt(session)

	if !strings.Contains(prompt, "+ 10 more tool calls") {
		t.Error("expected '+ 10 more tool calls' for 40 tool calls")
	}
}

func TestExtractReasoningSuccess(t *testing.T) {
	llmResponse := storage.LLMReasoning{
		Summary: "Added token bucket rate limiter to auth endpoint",
		FileAnnotations: []storage.FileAnnotation{
			{Path: "auth/middleware.go", What: "added rate limiter", Why: "prevent abuse"},
		},
		RejectedApproaches: []string{"sliding window - requires Lua scripts"},
		KeyDecisions:       []string{"chose token bucket for simplicity"},
	}
	respJSON, _ := json.Marshal(llmResponse)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			Choices: []Choice{
				{Message: ChatMessage{Role: "assistant", Content: string(respJSON)}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model", 5*time.Second)
	result, err := ExtractReasoning(context.Background(), client, testSession())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "Added token bucket rate limiter to auth endpoint" {
		t.Errorf("unexpected summary: %s", result.Summary)
	}
	if len(result.FileAnnotations) != 1 {
		t.Errorf("expected 1 file annotation, got %d", len(result.FileAnnotations))
	}
	if len(result.RejectedApproaches) != 1 {
		t.Errorf("expected 1 rejected approach, got %d", len(result.RejectedApproaches))
	}
	if len(result.KeyDecisions) != 1 {
		t.Errorf("expected 1 key decision, got %d", len(result.KeyDecisions))
	}
}

func TestExtractReasoningMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			Choices: []Choice{
				{Message: ChatMessage{Role: "assistant", Content: "not valid json"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model", 5*time.Second)
	_, err := ExtractReasoning(context.Background(), client, testSession())

	if err == nil {
		t.Fatal("expected error for malformed LLM response")
	}
}

func TestAbbreviateToolCall(t *testing.T) {
	tests := []struct {
		name string
		tc   storage.ToolCall
		want string
	}{
		{"read", storage.ToolCall{Tool: "Read", Input: json.RawMessage(`{"file_path":"foo.go"}`)}, "Read foo.go"},
		{"edit", storage.ToolCall{Tool: "Edit", Input: json.RawMessage(`{"file_path":"bar.go"}`)}, "Edit bar.go"},
		{"bash", storage.ToolCall{Tool: "Bash", Input: json.RawMessage(`{"command":"go test"}`)}, "Bash: go test"},
		{"glob", storage.ToolCall{Tool: "Glob", Input: json.RawMessage(`{"pattern":"*.go"}`)}, "Glob *.go"},
		{"unknown", storage.ToolCall{Tool: "Agent", Input: json.RawMessage(`{}`)}, "Agent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := abbreviateToolCall(tt.tc)
			if got != tt.want {
				t.Errorf("abbreviateToolCall() = %q, want %q", got, tt.want)
			}
		})
	}
}
