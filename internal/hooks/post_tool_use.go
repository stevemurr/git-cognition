package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/stevemurr/git-cognition/internal/capture"
)

type PostToolUsePayload struct {
	SessionID    string          `json:"session_id"`
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
	CWD          string          `json:"cwd"`
}

func RunPostToolUse(stdin io.Reader) error {
	gitDir, err := findGitDir()
	if err != nil {
		return nil // not in a git repo, silently skip
	}

	// Check if capture is enabled
	if _, err := os.Stat(filepath.Join(gitDir, "gc-enabled")); os.IsNotExist(err) {
		return nil
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("post-tool-use: read stdin: %w", err)
	}

	var payload PostToolUsePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("post-tool-use: parse payload: %w", err)
	}

	if payload.SessionID == "" {
		return nil
	}

	// Ensure session file exists with a start event
	sessionPath := capture.SessionFilePath(gitDir, payload.SessionID)
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		startEvent := &capture.Event{
			Type:      capture.EventSessionStart,
			SessionID: payload.SessionID,
			StartedAt: time.Now().UTC().Format(time.RFC3339),
			CWD:       payload.CWD,
		}
		if err := capture.AppendEvent(gitDir, payload.SessionID, startEvent); err != nil {
			return err
		}
	}

	// Convert tool_response to string for storage
	responseStr := stringifyJSON(payload.ToolResponse)

	event := &capture.Event{
		Type:      capture.EventToolCall,
		Tool:      payload.ToolName,
		Input:     payload.ToolInput,
		Output:    responseStr,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return capture.AppendEvent(gitDir, payload.SessionID, event)
}

// stringifyJSON converts a json.RawMessage to a readable string.
// If it's already a string, unquote it. Otherwise marshal it compactly.
func stringifyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Otherwise return the raw JSON
	return string(raw)
}

func findGitDir() (string, error) {
	// Walk up from cwd to find .git
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return gitPath, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}
