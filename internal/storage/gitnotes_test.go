package storage

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}
	return dir
}

func makeCommit(t *testing.T, dir, filename, msg string) string {
	t.Helper()
	os.WriteFile(filepath.Join(dir, filename), []byte("content"), 0o644)
	cmd := exec.Command("git", "add", filename)
	cmd.Dir = dir
	cmd.CombinedOutput()
	cmd = exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir
	cmd.CombinedOutput()

	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, _ := cmd.Output()
	return string(out[:len(out)-1]) // trim newline
}

func TestWriteAndReadSession(t *testing.T) {
	dir := setupGitRepo(t)
	sha := makeCommit(t, dir, "test.go", "initial commit")

	// Change to the git repo so git commands work
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	now := time.Now().UTC().Truncate(time.Second)
	session := &Session{
		SchemaVersion:  SchemaVersion,
		SessionID:      "test-session-1",
		CreatedAt:      now,
		CompletedAt:    now.Add(60 * time.Second),
		Agent:          Agent{Runner: "claude-code", Model: "claude-sonnet-4-6"},
		Task:           Task{Prompt: "Fix the bug"},
		Commits:        []Commit{{SHA: sha, Message: "initial commit", FilesChanged: []string{"test.go"}}},
		ToolCalls:      []ToolCall{{Sequence: 1, Tool: "Read", Input: json.RawMessage(`{}`), OutputTruncated: "content", Timestamp: now}},
		Reasoning:      Reasoning{FinalMessage: "I fixed the bug.", Source: "claude_final_message"},
		ThinkingBlocks: []json.RawMessage{},
		Metrics:        Metrics{ToolCallCount: 1, DurationSeconds: 60},
	}

	if err := WriteSession(session); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}

	// Read back by ID
	restored, err := ReadSessionByID("test-session-1")
	if err != nil {
		t.Fatalf("ReadSessionByID: %v", err)
	}
	if restored.SessionID != "test-session-1" {
		t.Errorf("session_id = %q", restored.SessionID)
	}
	if restored.Reasoning.FinalMessage != "I fixed the bug." {
		t.Errorf("final_message = %q", restored.Reasoning.FinalMessage)
	}

	// Read session ID from commit note
	noteID, err := ReadSessionIDForCommit(sha)
	if err != nil {
		t.Fatalf("ReadSessionIDForCommit: %v", err)
	}
	if noteID != "test-session-1" {
		t.Errorf("note session_id = %q", noteID)
	}
}

func TestListSessionRefs(t *testing.T) {
	dir := setupGitRepo(t)
	makeCommit(t, dir, "dummy.txt", "dummy")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	now := time.Now().UTC()
	for _, id := range []string{"session-a", "session-b"} {
		s := &Session{
			SchemaVersion:  SchemaVersion,
			SessionID:      id,
			CreatedAt:      now,
			CompletedAt:    now,
			Commits:        []Commit{},
			ToolCalls:      []ToolCall{},
			ThinkingBlocks: []json.RawMessage{},
		}
		if err := WriteSession(s); err != nil {
			t.Fatalf("WriteSession %s: %v", id, err)
		}
	}

	ids, err := ListSessionRefs()
	if err != nil {
		t.Fatalf("ListSessionRefs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("got %d refs, want 2: %v", len(ids), ids)
	}
}
