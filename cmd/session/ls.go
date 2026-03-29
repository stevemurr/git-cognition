package session

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/internal/output"
	"github.com/stevemurr/git-cognition/internal/storage"
)

var (
	lsFile  string
	lsSince string
	lsModel string
	lsLimit int
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List captured sessions",
	RunE:  runLs,
}

func init() {
	lsCmd.Flags().StringVar(&lsFile, "file", "", "filter by file path")
	lsCmd.Flags().StringVar(&lsSince, "since", "", "filter by duration (e.g. 7d, 30d)")
	lsCmd.Flags().StringVar(&lsModel, "model", "", "filter by model substring")
	lsCmd.Flags().IntVar(&lsLimit, "limit", 0, "max sessions to show")
	SessionCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
	ids, err := storage.ListSessionRefs()
	if err != nil {
		return err
	}

	var sinceTime time.Time
	if lsSince != "" {
		d, err := parseDuration(lsSince)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}
		sinceTime = time.Now().Add(-d)
	}

	var sessions []*storage.Session
	for _, id := range ids {
		s, err := storage.ReadSessionByID(id)
		if err != nil {
			continue
		}

		// Apply filters
		if !sinceTime.IsZero() && s.CreatedAt.Before(sinceTime) {
			continue
		}
		if lsModel != "" && !strings.Contains(strings.ToLower(s.Agent.Model), strings.ToLower(lsModel)) {
			continue
		}
		if lsFile != "" && !sessionTouchesFile(s, lsFile) {
			continue
		}

		sessions = append(sessions, s)

		if lsLimit > 0 && len(sessions) >= lsLimit {
			break
		}
	}

	if jsonOutput {
		output.RenderSessionListJSON(os.Stdout, sessions)
	} else {
		output.RenderSessionList(os.Stdout, sessions)
	}
	return nil
}

func sessionTouchesFile(s *storage.Session, file string) bool {
	for _, c := range s.Commits {
		for _, f := range c.FilesChanged {
			if strings.Contains(f, file) {
				return true
			}
		}
	}
	return false
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
