package session

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/internal/output"
	"github.com/stevemurr/git-cognition/internal/storage"
)

var statCmd = &cobra.Command{
	Use:   "stat",
	Short: "Show session statistics",
	RunE:  runStat,
}

func init() {
	SessionCmd.AddCommand(statCmd)
}

func runStat(cmd *cobra.Command, args []string) error {
	ids, err := storage.ListSessionRefs()
	if err != nil {
		return err
	}

	var sessions []*storage.Session
	for _, id := range ids {
		s, err := storage.ReadSessionByID(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	output.RenderSessionStat(os.Stdout, sessions)
	return nil
}
