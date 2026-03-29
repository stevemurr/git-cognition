package capture

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/stevemurr/git-cognition/internal/storage"
)

// FindCommits returns commits made between start and end times.
func FindCommits(start, end time.Time) ([]storage.Commit, error) {
	afterArg := fmt.Sprintf("--after=%s", start.Format(time.RFC3339))
	beforeArg := fmt.Sprintf("--before=%s", end.Add(time.Second).Format(time.RFC3339))

	// Use NUL-separated format to avoid tab/newline ambiguity
	// Each commit: <sha> <subject>\n<file1>\n<file2>\n...
	cmd := exec.Command("git", "log", afterArg, beforeArg,
		"--format=%h %s", "--name-only")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("capture: git log: %w", err)
	}

	return parseGitLog(string(out)), nil
}

func parseGitLog(output string) []storage.Commit {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	// git log --name-only outputs:
	//   <sha> <subject>
	//   <blank line>
	//   <file1>
	//   <file2>
	//   <blank line>  (separator before next commit)
	//   <sha> <subject>
	//   ...
	//
	// We split on double-newline to get blocks, then merge pairs:
	// the first block is the header, the second is the file list.
	// But some commits have no changed files, giving empty blocks.

	var commits []storage.Commit
	lines := strings.Split(output, "\n")

	var current *storage.Commit
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if line == "" {
			// Blank line — if we have a current commit, files section follows
			// or this is a separator between commits
			continue
		}

		// Try to parse as a commit header: <sha> <subject>
		// SHA is 7+ hex chars followed by a space
		if len(line) > 8 && line[7] == ' ' && isHex(line[:7]) {
			// Save previous commit
			if current != nil {
				if current.FilesChanged == nil {
					current.FilesChanged = []string{}
				}
				commits = append(commits, *current)
			}
			sha := line[:7]
			msg := line[8:]
			current = &storage.Commit{
				SHA:     sha,
				Message: msg,
			}
		} else if current != nil {
			// It's a filename belonging to the current commit
			current.FilesChanged = append(current.FilesChanged, line)
		}
	}

	// Don't forget the last commit
	if current != nil {
		if current.FilesChanged == nil {
			current.FilesChanged = []string{}
		}
		commits = append(commits, *current)
	}

	return commits
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
