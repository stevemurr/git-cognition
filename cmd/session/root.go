package session

import (
	"github.com/spf13/cobra"
)

var jsonOutput bool

var SessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage and query captured sessions",
}

func init() {
	SessionCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
}
