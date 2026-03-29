package storage

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// WriteSession stores the session JSON as a blob at refs/sessions/<session_id>
// and annotates each commit in the session with the session ID via git notes.
func WriteSession(s *Session) error {
	data, err := MarshalSession(s)
	if err != nil {
		return fmt.Errorf("gitnotes: marshal session: %w", err)
	}

	// Create blob from session JSON
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gitnotes: hash-object: %w", err)
	}
	blobSHA := strings.TrimSpace(string(out))

	// Create ref pointing to the blob
	ref := fmt.Sprintf("refs/sessions/%s", s.SessionID)
	if err := gitExec("update-ref", ref, blobSHA); err != nil {
		return fmt.Errorf("gitnotes: update-ref %s: %w", ref, err)
	}

	// Annotate each commit with the session ID
	for _, c := range s.Commits {
		// Use -f to overwrite if note already exists
		if err := gitExec("notes", "--ref=sessions", "add", "-f", "-m", s.SessionID, c.SHA); err != nil {
			// Non-fatal: commit may not exist yet or may be from another repo
			continue
		}
	}

	return nil
}

// ReadSessionByID loads a session by reading the blob at refs/sessions/<session_id>.
func ReadSessionByID(sessionID string) (*Session, error) {
	ref := fmt.Sprintf("refs/sessions/%s", sessionID)
	cmd := exec.Command("git", "cat-file", "blob", ref)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gitnotes: cat-file %s: %w", ref, err)
	}
	return UnmarshalSession(out)
}

// ReadSessionIDForCommit returns the session ID noted on a commit.
func ReadSessionIDForCommit(sha string) (string, error) {
	cmd := exec.Command("git", "notes", "--ref=sessions", "show", sha)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gitnotes: no session note on %s: %w", sha, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ListSessionRefs returns all session IDs stored under refs/sessions/.
func ListSessionRefs() ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/sessions/")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gitnotes: for-each-ref: %w", err)
	}

	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// refname:short gives "sessions/<id>", strip the prefix
		if strings.HasPrefix(line, "sessions/") {
			ids = append(ids, strings.TrimPrefix(line, "sessions/"))
		} else {
			ids = append(ids, line)
		}
	}
	return ids, nil
}

func gitExec(args ...string) error {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}
