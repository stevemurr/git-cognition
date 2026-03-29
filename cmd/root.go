package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/cmd/session"
)

var Version = "dev"

var (
	jsonOutput bool
	noColor    bool
)

var rootCmd = &cobra.Command{
	Use:   "git-cognition",
	Short: "Capture and query AI coding session context via git notes",
	Long:  "git-cognition stores Claude Code session data (tool calls, reasoning) in git notes and lets you query why any line of code was written.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if noColor {
			color.NoColor = true
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("git-cognition", Version)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.AddCommand(session.SessionCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
