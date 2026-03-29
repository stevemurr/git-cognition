package capture

import (
	"testing"
)

func TestParseGitLog(t *testing.T) {
	output := `8b2e4f3	feat: token bucket rate limiter on /auth
auth/middleware.py
tests/test_auth.py

a1b2c3d	fix: handle nil pointer in auth
auth/middleware.py`

	commits := parseGitLog(output)
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}

	if commits[0].SHA != "8b2e4f3" {
		t.Errorf("commit[0].sha = %q", commits[0].SHA)
	}
	if commits[0].Message != "feat: token bucket rate limiter on /auth" {
		t.Errorf("commit[0].message = %q", commits[0].Message)
	}
	if len(commits[0].FilesChanged) != 2 {
		t.Errorf("commit[0].files = %d, want 2", len(commits[0].FilesChanged))
	}

	if commits[1].SHA != "a1b2c3d" {
		t.Errorf("commit[1].sha = %q", commits[1].SHA)
	}
	if len(commits[1].FilesChanged) != 1 {
		t.Errorf("commit[1].files = %d, want 1", len(commits[1].FilesChanged))
	}
}

func TestParseGitLogEmpty(t *testing.T) {
	commits := parseGitLog("")
	if commits != nil {
		t.Errorf("expected nil, got %v", commits)
	}
}
