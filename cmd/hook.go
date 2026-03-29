package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/internal/hooks"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Hook entry points for Claude Code",
}

var postToolUseCmd = &cobra.Command{
	Use:   "post-tool-use",
	Short: "Handle PostToolUse hook event",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := hooks.RunPostToolUse(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil // always exit 0
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Handle Stop hook event",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := hooks.RunStop(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil // always exit 0
	},
}

func init() {
	hookCmd.AddCommand(postToolUseCmd)
	hookCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(hookCmd)
}
