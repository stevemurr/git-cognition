package session

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/internal/output"
	"github.com/stevemurr/git-cognition/internal/storage"
)

var (
	grepScope string
	grepSince string
)

var grepCmd = &cobra.Command{
	Use:   "grep <query>",
	Short: "Search session content",
	Long:  "Searches final_message by default. Use --scope to search tool calls or all content.",
	Args:  cobra.ExactArgs(1),
	RunE:  runGrep,
}

func init() {
	grepCmd.Flags().StringVar(&grepScope, "scope", "reasoning", "search scope: reasoning, tools, all")
	grepCmd.Flags().StringVar(&grepSince, "since", "", "filter by duration (e.g. 7d)")
	SessionCmd.AddCommand(grepCmd)
}

func runGrep(cmd *cobra.Command, args []string) error {
	query := strings.ToLower(args[0])

	ids, err := storage.ListSessionRefs()
	if err != nil {
		return err
	}

	var sinceTime time.Time
	if grepSince != "" {
		d, err := parseDuration(grepSince)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}
		sinceTime = time.Now().Add(-d)
	}

	var results []output.GrepResult
	for _, id := range ids {
		s, err := storage.ReadSessionByID(id)
		if err != nil {
			continue
		}

		if !sinceTime.IsZero() && s.CreatedAt.Before(sinceTime) {
			continue
		}

		var matches []string

		// Search reasoning (final_message)
		if grepScope == "reasoning" || grepScope == "all" {
			if strings.Contains(strings.ToLower(s.Reasoning.FinalMessage), query) {
				matches = append(matches, findMatchingLines(s.Reasoning.FinalMessage, query)...)
			}
		}

		// Search tool calls
		if grepScope == "tools" || grepScope == "all" {
			for _, tc := range s.ToolCalls {
				inputStr := string(tc.Input)
				if strings.Contains(strings.ToLower(inputStr), query) {
					var input map[string]interface{}
					json.Unmarshal(tc.Input, &input)
					matches = append(matches, fmt.Sprintf("[%s] %v", tc.Tool, input))
				}
				if strings.Contains(strings.ToLower(tc.OutputTruncated), query) {
					matches = append(matches, fmt.Sprintf("[%s output] ...%s...", tc.Tool, query))
				}
			}
		}

		if len(matches) > 0 {
			results = append(results, output.GrepResult{Session: s, Matches: matches})
		}
	}

	if jsonOutput {
		output.RenderSessionGrepJSON(os.Stdout, results)
	} else {
		output.RenderSessionGrep(os.Stdout, query, results)
	}
	return nil
}

func findMatchingLines(text, query string) []string {
	var matches []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, strings.TrimSpace(line))
		}
	}
	return matches
}
