package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/stevemurr/git-cognition/internal/capture"
	"github.com/stevemurr/git-cognition/internal/config"
	"github.com/stevemurr/git-cognition/internal/llm"
	"github.com/stevemurr/git-cognition/internal/storage"
)

type StopPayload struct {
	SessionID            string `json:"session_id"`
	CWD                  string `json:"cwd"`
	TranscriptPath       string `json:"transcript_path"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

func RunStop(stdin io.Reader) error {
	gitDir, err := findGitDir()
	if err != nil {
		return nil
	}

	if _, err := os.Stat(filepath.Join(gitDir, "gc-enabled")); os.IsNotExist(err) {
		return nil
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("stop: read stdin: %w", err)
	}

	var payload StopPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("stop: parse payload: %w", err)
	}

	if payload.SessionID == "" {
		return nil
	}

	// Read events from session file
	events, _ := capture.ReadEvents(gitDir, payload.SessionID)

	// Extract final message: prefer last_assistant_message from payload,
	// fall back to reading the transcript file
	finalMessage := payload.LastAssistantMessage
	var taskPrompt string
	var model string

	if payload.TranscriptPath != "" {
		transcriptData, err := os.ReadFile(payload.TranscriptPath)
		if err == nil {
			if finalMessage == "" {
				finalMessage, _ = capture.ExtractFinalMessage(transcriptData)
			}
			taskPrompt, _ = capture.ExtractTaskPrompt(transcriptData)
			model = capture.ExtractModel(transcriptData)
		}
	}

	// Determine reasoning source
	source := "none"
	if finalMessage != "" {
		source = "claude_final_message"
	}

	// Parse timestamps from events
	var startTime, endTime time.Time
	now := time.Now().UTC()
	endTime = now

	for _, ev := range events {
		if ev.Type == capture.EventSessionStart && ev.StartedAt != "" {
			if t, err := time.Parse(time.RFC3339, ev.StartedAt); err == nil {
				startTime = t
			}
		}
	}
	if startTime.IsZero() {
		startTime = now.Add(-5 * time.Minute) // fallback
	}

	// Find commits made during session
	commits, _ := capture.FindCommits(startTime, endTime)

	// Build tool calls with sequence numbers
	var toolCalls []storage.ToolCall
	seq := 0
	for _, ev := range events {
		if ev.Type != capture.EventToolCall {
			continue
		}
		seq++
		ts, _ := time.Parse(time.RFC3339, ev.Timestamp)
		toolCalls = append(toolCalls, storage.ToolCall{
			Sequence:        seq,
			Tool:            ev.Tool,
			Input:           ev.Input,
			OutputTruncated: ev.Output,
			Timestamp:       ts,
		})
	}

	// Assemble session
	session := &storage.Session{
		SchemaVersion:  storage.SchemaVersion,
		SessionID:      payload.SessionID,
		CreatedAt:      startTime,
		CompletedAt:    endTime,
		Agent:          storage.Agent{Runner: "claude-code", Model: model},
		Task:           storage.Task{Prompt: taskPrompt},
		Commits:        commits,
		ToolCalls:      toolCalls,
		Reasoning:      storage.Reasoning{FinalMessage: finalMessage, Source: source},
		ThinkingBlocks: []json.RawMessage{},
		Metrics:        storage.Metrics{ToolCallCount: len(toolCalls), DurationSeconds: int(endTime.Sub(startTime).Seconds())},
	}

	if session.Commits == nil {
		session.Commits = []storage.Commit{}
	}

	// Attempt LLM reasoning extraction
	cfg, _ := config.Load(gitDir)
	if cfg.LLM.Enabled && cfg.LLM.Endpoint != "" {
		apiKey := cfg.LLM.APIKey
		if envKey := os.Getenv("GC_LLM_API_KEY"); envKey != "" {
			apiKey = envKey
		}
		client := llm.NewClient(cfg.LLM.Endpoint, apiKey, cfg.LLM.Model,
			time.Duration(cfg.LLM.TimeoutS)*time.Second)
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(cfg.LLM.TimeoutS)*time.Second)
		defer cancel()
		if extracted, err := llm.ExtractReasoning(ctx, client, session); err == nil {
			session.Reasoning.LLM = extracted
			session.Reasoning.Source = "llm_extracted"
		}
	}

	// Write to git notes
	if err := storage.WriteSession(session); err != nil {
		return fmt.Errorf("stop: write session: %w", err)
	}

	// Clean up session file
	capture.Remove(gitDir, payload.SessionID)

	// Clean up orphaned sessions (best effort)
	go cleanupOrphans(gitDir)

	return nil
}

func cleanupOrphans(gitDir string) {
	orphans, err := capture.ListOrphanedSessions(gitDir, 24*time.Hour)
	if err != nil {
		return
	}
	for _, sid := range orphans {
		capture.Remove(gitDir, sid)
	}
}
