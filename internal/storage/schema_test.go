package storage

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &Session{
		SchemaVersion: SchemaVersion,
		SessionID:     "abc123",
		CreatedAt:     now,
		CompletedAt:   now.Add(107 * time.Second),
		Agent:         Agent{Runner: "claude-code", Model: "claude-sonnet-4-6"},
		Task:          Task{Prompt: "Add rate limiting to the auth endpoint"},
		Commits: []Commit{
			{SHA: "8b2e4f3", Message: "feat: token bucket rate limiter on /auth", FilesChanged: []string{"auth/middleware.py", "tests/test_auth.py"}},
		},
		ToolCalls: []ToolCall{
			{Sequence: 1, Tool: "Read", Input: json.RawMessage(`{"file_path":"redis_client.py"}`), OutputTruncated: "# Redis connection pool", Timestamp: now},
			{Sequence: 2, Tool: "Edit", Input: json.RawMessage(`{"file_path":"auth/middleware.py"}`), OutputTruncated: "Edit applied successfully", Timestamp: now},
		},
		Reasoning: Reasoning{
			FinalMessage: "I've added a token bucket rate limiter.",
			Source:       "claude_final_message",
		},
		ThinkingBlocks: []json.RawMessage{},
		Metrics:        Metrics{ToolCallCount: 2, DurationSeconds: 107},
	}

	data, err := MarshalSession(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	restored, err := UnmarshalSession(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", restored.SchemaVersion, SchemaVersion)
	}
	if restored.SessionID != original.SessionID {
		t.Errorf("session_id = %q, want %q", restored.SessionID, original.SessionID)
	}
	if restored.Agent.Model != original.Agent.Model {
		t.Errorf("agent.model = %q, want %q", restored.Agent.Model, original.Agent.Model)
	}
	if restored.Reasoning.FinalMessage != original.Reasoning.FinalMessage {
		t.Errorf("reasoning.final_message mismatch")
	}
	if len(restored.ThinkingBlocks) != 0 {
		t.Errorf("thinking_blocks should be empty, got %d", len(restored.ThinkingBlocks))
	}
	if len(restored.Commits) != 1 {
		t.Errorf("commits count = %d, want 1", len(restored.Commits))
	}
	if len(restored.ToolCalls) != 2 {
		t.Errorf("tool_calls count = %d, want 2", len(restored.ToolCalls))
	}
}

func TestThinkingBlocksSerializesAsEmptyArray(t *testing.T) {
	s := &Session{
		SchemaVersion:  SchemaVersion,
		SessionID:      "test",
		Commits:        []Commit{},
		ToolCalls:      []ToolCall{},
		ThinkingBlocks: []json.RawMessage{},
	}
	data, err := MarshalSession(s)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	if string(raw["thinking_blocks"]) != "[]" {
		t.Errorf("thinking_blocks = %s, want []", raw["thinking_blocks"])
	}
}

func TestParseToolCall(t *testing.T) {
	tests := []struct {
		name string
		tc   ToolCall
		want ToolCallInfo
	}{
		{"read", ToolCall{Tool: "Read", Input: json.RawMessage(`{"file_path":"foo.go"}`)}, ToolCallInfo{Tool: "Read", FilePath: "foo.go"}},
		{"bash", ToolCall{Tool: "Bash", Input: json.RawMessage(`{"command":"go test"}`)}, ToolCallInfo{Tool: "Bash", Command: "go test"}},
		{"glob", ToolCall{Tool: "Glob", Input: json.RawMessage(`{"pattern":"*.go"}`)}, ToolCallInfo{Tool: "Glob", Pattern: "*.go"}},
		{"unknown", ToolCall{Tool: "Agent", Input: json.RawMessage(`{}`)}, ToolCallInfo{Tool: "Agent"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseToolCall(tt.tc)
			if got != tt.want {
				t.Errorf("ParseToolCall() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
