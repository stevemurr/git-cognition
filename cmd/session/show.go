package session

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/internal/output"
	"github.com/stevemurr/git-cognition/internal/storage"
)

var showCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show details of a captured session",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	SessionCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	session, err := storage.ReadSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("session %q not found: %w", sessionID, err)
	}

	if jsonOutput {
		output.RenderSessionShowJSON(os.Stdout, session)
	} else {
		output.RenderSessionShow(os.Stdout, session)
	}
	return nil
}
