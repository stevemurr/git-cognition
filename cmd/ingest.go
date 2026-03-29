package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/internal/storage"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest session data from NDJSON on stdin",
	Long:  "Reads NDJSON events from stdin and writes a session to git notes. Agent-agnostic capture path.",
	RunE:  runIngest,
}

func init() {
	rootCmd.AddCommand(ingestCmd)
}

type ingestEvent struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	StartedAt string          `json:"started_at,omitempty"`
	Task      string          `json:"task,omitempty"`
	Model     string          `json:"model,omitempty"`
	Runner    string          `json:"runner,omitempty"`
	CWD       string          `json:"cwd,omitempty"`
	Sequence  int             `json:"sequence,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    string          `json:"output,omitempty"`
	Content   string          `json:"content,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	Metrics   *ingestMetrics  `json:"metrics,omitempty"`
}

type ingestMetrics struct {
	DurationSeconds int `json:"duration_seconds"`
}

func runIngest(cmd *cobra.Command, args []string) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		sessionID    string
		startedAt    time.Time
		task         string
		model        string
		runner       string
		toolCalls    []storage.ToolCall
		finalMessage string
		hasFinal     bool
		endTime      time.Time
		duration     int
	)

	seq := 0
	for scanner.Scan() {
		var ev ingestEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue // skip malformed lines
		}

		switch ev.Type {
		case "session_start":
			sessionID = ev.SessionID
			if ev.StartedAt != "" {
				startedAt, _ = time.Parse(time.RFC3339, ev.StartedAt)
			}
			task = ev.Task
			model = ev.Model
			runner = ev.Runner

		case "tool_call":
			seq++
			s := ev.Sequence
			if s == 0 {
				s = seq
			}
			ts, _ := time.Parse(time.RFC3339, ev.Timestamp)
			output := ev.Output
			if len(output) > 4000 {
				output = output[:4000]
			}
			toolCalls = append(toolCalls, storage.ToolCall{
				Sequence:        s,
				Tool:            ev.Tool,
				Input:           ev.Input,
				OutputTruncated: output,
				Timestamp:       ts,
			})

		case "final_message":
			finalMessage = ev.Content
			hasFinal = true

		case "session_end":
			if ev.Timestamp != "" {
				endTime, _ = time.Parse(time.RFC3339, ev.Timestamp)
			}
			if ev.Metrics != nil {
				duration = ev.Metrics.DurationSeconds
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ingest: read stdin: %w", err)
	}

	if sessionID == "" {
		return fmt.Errorf("ingest: no session_start event found")
	}

	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	if endTime.IsZero() {
		endTime = time.Now().UTC()
	}

	source := "none"
	if hasFinal {
		source = "ingest_provided"
	}

	session := &storage.Session{
		SchemaVersion:  storage.SchemaVersion,
		SessionID:      sessionID,
		CreatedAt:      startedAt,
		CompletedAt:    endTime,
		Agent:          storage.Agent{Runner: runner, Model: model},
		Task:           storage.Task{Prompt: task},
		Commits:        []storage.Commit{},
		ToolCalls:      toolCalls,
		Reasoning:      storage.Reasoning{FinalMessage: finalMessage, Source: source},
		ThinkingBlocks: []json.RawMessage{},
		Metrics:        storage.Metrics{ToolCallCount: len(toolCalls), DurationSeconds: duration},
	}

	if err := storage.WriteSession(session); err != nil {
		return fmt.Errorf("ingest: write session: %w", err)
	}

	fmt.Fprintf(os.Stderr, "ingested session %s (%d tool calls, source: %s)\n", sessionID, len(toolCalls), source)
	return nil
}
