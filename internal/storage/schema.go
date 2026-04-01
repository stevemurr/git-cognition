package storage

import (
	"encoding/json"
	"time"
)

const SchemaVersion = "6.0"

type Session struct {
	SchemaVersion string    `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	CreatedAt     time.Time `json:"created_at"`
	CompletedAt   time.Time `json:"completed_at"`
	Agent         Agent     `json:"agent"`
	Task          Task      `json:"task"`
	Commits       []Commit  `json:"commits"`
	ToolCalls     []ToolCall `json:"tool_calls"`
	Reasoning     Reasoning `json:"reasoning"`
	ThinkingBlocks []json.RawMessage `json:"thinking_blocks"`
	Metrics       Metrics   `json:"metrics"`
}

type Agent struct {
	Runner string `json:"runner"`
	Model  string `json:"model"`
}

type Task struct {
	Prompt string `json:"prompt"`
}

type Commit struct {
	SHA          string   `json:"sha"`
	Message      string   `json:"message"`
	FilesChanged []string `json:"files_changed"`
}

type ToolCall struct {
	Sequence        int             `json:"sequence"`
	Tool            string          `json:"tool"`
	Input           json.RawMessage `json:"input"`
	OutputTruncated string          `json:"output_truncated"`
	Timestamp       time.Time       `json:"timestamp"`
}

type Reasoning struct {
	FinalMessage string        `json:"final_message"`
	Source       string        `json:"source"`
	LLM          *LLMReasoning `json:"llm,omitempty"`
}

type LLMReasoning struct {
	Summary            string           `json:"summary"`
	FileAnnotations    []FileAnnotation `json:"file_annotations"`
	RejectedApproaches []string         `json:"rejected_approaches"`
	KeyDecisions       []string         `json:"key_decisions"`
}

type FileAnnotation struct {
	Path string `json:"path"`
	What string `json:"what"`
	Why  string `json:"why"`
}

type Metrics struct {
	ToolCallCount   int `json:"tool_call_count"`
	DurationSeconds int `json:"duration_seconds"`
}

func NewSession(sessionID string) *Session {
	return &Session{
		SchemaVersion:  SchemaVersion,
		SessionID:      sessionID,
		Commits:        []Commit{},
		ToolCalls:      []ToolCall{},
		ThinkingBlocks: []json.RawMessage{},
	}
}

func MarshalSession(s *Session) ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func UnmarshalSession(data []byte) (*Session, error) {
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Commits == nil {
		s.Commits = []Commit{}
	}
	if s.ToolCalls == nil {
		s.ToolCalls = []ToolCall{}
	}
	if s.ThinkingBlocks == nil {
		s.ThinkingBlocks = []json.RawMessage{}
	}
	return &s, nil
}
