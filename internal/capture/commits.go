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

	// Format: sha<TAB>subject<NUL>files...
	cmd := exec.Command("git", "log", afterArg, beforeArg, "--format=%h\t%s", "--name-only")
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

	var commits []storage.Commit
	// git log --name-only separates entries by blank lines
	blocks := strings.Split(strings.TrimSpace(output), "\n\n")
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) == 0 {
			continue
		}
		parts := strings.SplitN(lines[0], "\t", 2)
		if len(parts) < 2 {
			continue
		}
		c := storage.Commit{
			SHA:     parts[0],
			Message: parts[1],
		}
		for _, f := range lines[1:] {
			f = strings.TrimSpace(f)
			if f != "" {
				c.FilesChanged = append(c.FilesChanged, f)
			}
		}
		if c.FilesChanged == nil {
			c.FilesChanged = []string{}
		}
		commits = append(commits, c)
	}
	return commits
}
