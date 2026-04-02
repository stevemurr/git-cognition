package capture

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAndReadEvents(t *testing.T) {
	gitDir := t.TempDir()
	sid := "test-session"

	ev1 := &Event{
		Type:      EventSessionStart,
		SessionID: sid,
		StartedAt: "2026-03-27T14:33:00Z",
		CWD:       "/tmp/test",
	}
	ev2 := &Event{
		Type:      EventToolCall,
		Tool:      "Read",
		Input:     json.RawMessage(`{"file_path":"main.go"}`),
		Output:    "package main",
		Timestamp: "2026-03-27T14:33:22Z",
	}

	if err := AppendEvent(gitDir, sid, ev1); err != nil {
		t.Fatal(err)
	}
	if err := AppendEvent(gitDir, sid, ev2); err != nil {
		t.Fatal(err)
	}

	events, err := ReadEvents(gitDir, sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Type != EventSessionStart {
		t.Errorf("event[0].type = %q, want %q", events[0].Type, EventSessionStart)
	}
	if events[1].Tool != "Read" {
		t.Errorf("event[1].tool = %q, want Read", events[1].Tool)
	}
}

func TestOutputTruncation(t *testing.T) {
	gitDir := t.TempDir()
	sid := "trunc-session"

	longOutput := strings.Repeat("x", MaxOutputLen+500)
	ev := &Event{
		Type:   EventToolCall,
		Tool:   "Read",
		Output: longOutput,
	}

	if err := AppendEvent(gitDir, sid, ev); err != nil {
		t.Fatal(err)
	}

	events, err := ReadEvents(gitDir, sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(events[0].Output) != MaxOutputLen {
		t.Errorf("output length = %d, want %d", len(events[0].Output), MaxOutputLen)
	}
}

func TestRemove(t *testing.T) {
	gitDir := t.TempDir()
	sid := "rm-session"

	ev := &Event{Type: EventSessionStart, SessionID: sid}
	AppendEvent(gitDir, sid, ev)

	if err := Remove(gitDir, sid); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(sessionDir(gitDir), sid+".ndjson")
	if _, err := ReadEvents(gitDir, sid); err == nil {
		t.Errorf("expected error reading removed file at %s", path)
	}
}
