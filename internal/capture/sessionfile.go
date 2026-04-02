package capture

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const MaxOutputLen = 4000

type EventType string

const (
	EventSessionStart EventType = "session_start"
	EventToolCall     EventType = "tool_call"
)

type Event struct {
	Type      EventType       `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	StartedAt string          `json:"started_at,omitempty"`
	CWD       string          `json:"cwd,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    string          `json:"output,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
}

func sessionDir(gitDir string) string {
	return filepath.Join(gitDir, "gc-sessions")
}

func SessionFilePath(gitDir, sessionID string) string {
	return filepath.Join(sessionDir(gitDir), sessionID+".ndjson")
}

func AppendEvent(gitDir, sessionID string, event *Event) error {
	dir := sessionDir(gitDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("capture: create session dir: %w", err)
	}

	// Truncate output at write time
	if len(event.Output) > MaxOutputLen {
		event.Output = event.Output[:MaxOutputLen]
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("capture: marshal event: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(SessionFilePath(gitDir, sessionID), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("capture: open session file: %w", err)
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func ReadEvents(gitDir, sessionID string) ([]Event, error) {
	f, err := os.Open(SessionFilePath(gitDir, sessionID))
	if err != nil {
		return nil, fmt.Errorf("capture: open session file: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue // skip malformed lines
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

func Remove(gitDir, sessionID string) error {
	return os.Remove(SessionFilePath(gitDir, sessionID))
}

func ListOrphanedSessions(gitDir string, olderThan time.Duration) ([]string, error) {
	dir := sessionDir(gitDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	cutoff := time.Now().Add(-olderThan)
	var orphans []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			name := e.Name()
			if ext := filepath.Ext(name); ext == ".ndjson" {
				orphans = append(orphans, name[:len(name)-len(ext)])
			}
		}
	}
	return orphans, nil
}
