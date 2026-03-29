package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stevemurr/git-cognition/internal/capture"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}
	// Enable capture
	os.WriteFile(filepath.Join(dir, ".git", "gc-enabled"), []byte(""), 0o644)
	return dir
}

func TestPostToolUse(t *testing.T) {
	dir := setupTestRepo(t)
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	payload := `{
		"session_id": "test-123",
		"tool_name": "Read",
		"tool_input": {"file_path": "main.go"},
		"tool_response": "package main",
		"cwd": "/tmp/test"
	}`

	if err := RunPostToolUse(strings.NewReader(payload)); err != nil {
		t.Fatalf("RunPostToolUse: %v", err)
	}

	events, err := capture.ReadEvents(filepath.Join(dir, ".git"), "test-123")
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}

	if len(events) != 2 { // session_start + tool_call
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Type != capture.EventSessionStart {
		t.Errorf("event[0].type = %q", events[0].Type)
	}
	if events[1].Tool != "Read" {
		t.Errorf("event[1].tool = %q", events[1].Tool)
	}
}

func TestPostToolUseSkipsWithoutGcEnabled(t *testing.T) {
	dir := t.TempDir()
	// Init git but don't create gc-enabled
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.CombinedOutput()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	payload := `{"session_id": "test-123", "tool_name": "Read", "tool_input": {}, "tool_response": "x", "cwd": "."}`
	if err := RunPostToolUse(strings.NewReader(payload)); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// No session file should exist
	if _, err := os.Stat(capture.SessionFilePath(filepath.Join(dir, ".git"), "test-123")); !os.IsNotExist(err) {
		t.Error("session file should not exist without gc-enabled")
	}
}
