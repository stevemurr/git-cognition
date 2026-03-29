package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stevemurr/git-cognition/internal/capture"
	"github.com/stevemurr/git-cognition/internal/storage"
)

func TestRunStop(t *testing.T) {
	dir := setupTestRepo(t)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Make a commit so git notes can attach
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0o644)
	for _, args := range [][]string{
		{"git", "add", "test.go"},
		{"git", "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.CombinedOutput()
	}

	gitDir := filepath.Join(dir, ".git")

	// Write some tool call events
	capture.AppendEvent(gitDir, "stop-test", &capture.Event{
		Type:      capture.EventSessionStart,
		SessionID: "stop-test",
		StartedAt: "2026-03-27T14:33:00Z",
		CWD:       dir,
	})
	capture.AppendEvent(gitDir, "stop-test", &capture.Event{
		Type:      capture.EventToolCall,
		Tool:      "Read",
		Output:    "package main",
		Timestamp: "2026-03-27T14:33:22Z",
	})

	// Write a transcript JSONL file
	transcriptPath := filepath.Join(t.TempDir(), "transcript.jsonl")
	transcriptContent := `{"role": "user", "content": "Fix the bug"}
{"role": "assistant", "content": [{"type": "text", "text": "I fixed the bug by updating the handler."}]}
`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0o644)

	payload := `{
		"session_id": "stop-test",
		"cwd": "` + dir + `",
		"transcript_path": "` + transcriptPath + `"
	}`

	if err := RunStop(strings.NewReader(payload)); err != nil {
		t.Fatalf("RunStop: %v", err)
	}

	// Session file should be removed
	if _, err := os.Stat(capture.SessionFilePath(gitDir, "stop-test")); !os.IsNotExist(err) {
		t.Error("session file should be deleted after stop")
	}

	// Session should be readable from git notes
	session, err := storage.ReadSessionByID("stop-test")
	if err != nil {
		t.Fatalf("ReadSessionByID: %v", err)
	}
	if session.Reasoning.FinalMessage != "I fixed the bug by updating the handler." {
		t.Errorf("final_message = %q", session.Reasoning.FinalMessage)
	}
	if session.Reasoning.Source != "claude_final_message" {
		t.Errorf("source = %q", session.Reasoning.Source)
	}
	if session.Task.Prompt != "Fix the bug" {
		t.Errorf("task.prompt = %q", session.Task.Prompt)
	}
	if len(session.ToolCalls) != 1 {
		t.Errorf("tool_calls = %d, want 1", len(session.ToolCalls))
	}
	if session.ToolCalls[0].Sequence != 1 {
		t.Errorf("tool_calls[0].sequence = %d, want 1", session.ToolCalls[0].Sequence)
	}
}
