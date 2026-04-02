package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/stevemurr/git-cognition/internal/storage"
)

func RenderSessionList(w io.Writer, sessions []*storage.Session) {
	fmt.Fprintf(w, "%s\n",
		HeaderDim(fmt.Sprintf("%-10s %-12s %-22s %5s  %s", "SESSION", "DATE", "MODEL", "FILES", "TASK")))
	for _, s := range sessions {
		fileCount := 0
		for _, c := range s.Commits {
			fileCount += len(c.FilesChanged)
		}
		task := s.Task.Prompt
		if len(task) > 50 {
			task = task[:47] + "..."
		}
		fmt.Fprintf(w, "%-10s %-12s %-22s %5d  %s\n",
			SHA(truncID(s.SessionID)),
			DateDim(s.CreatedAt.Format("2006-01-02")),
			Model(s.Agent.Model),
			fileCount, task)
	}
	fmt.Fprintf(w, "%s\n", Separator(strings.Repeat("─", 70)))
	fmt.Fprintf(w, "%s\n", HintText(fmt.Sprintf("%d sessions  ·  git session show <id> for detail", len(sessions))))
}

func RenderSessionListJSON(w io.Writer, sessions []*storage.Session) {
	data, _ := json.MarshalIndent(sessions, "", "  ")
	fmt.Fprintln(w, string(data))
}

func RenderSessionShow(w io.Writer, s *storage.Session) {
	dur := s.CompletedAt.Sub(s.CreatedAt)
	fmt.Fprintf(w, "%s  %s  %s  %s  %s  %s  %s  %s\n",
		SHA(s.SessionID),
		Separator("·"), Model(s.Agent.Model),
		Separator("·"), DateDim(s.CreatedAt.Format("2006-01-02 15:04")),
		Separator("·"), DateDim(formatDuration(dur)),
		"")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s    %s\n", Label("task:"), s.Task.Prompt)
	fmt.Fprintln(w)

	if len(s.Commits) > 0 {
		fmt.Fprintln(w, Header("commits:"))
		for _, c := range s.Commits {
			fmt.Fprintf(w, "  %s  %s\n", SHA(c.SHA), c.Message)
		}
		fmt.Fprintln(w)
	}

	if s.Reasoning.FinalMessage != "" {
		fmt.Fprintln(w, Header("claude's reasoning:"))
		renderQuotedMessage(w, s.Reasoning.FinalMessage)
		fmt.Fprintln(w)
	}

	if len(s.ToolCalls) > 0 {
		fmt.Fprintln(w, Header("action log:"))
		for _, tc := range s.ToolCalls {
			fmt.Fprintf(w, "  %s  %s\n", Number(fmt.Sprintf("%d.", tc.Sequence)), formatToolCallShort(tc))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, HintText("  git why <file>:<line> to trace a specific line"))
}

func RenderSessionShowJSON(w io.Writer, s *storage.Session) {
	data, _ := json.MarshalIndent(s, "", "  ")
	fmt.Fprintln(w, string(data))
}

func RenderSessionStat(w io.Writer, sessions []*storage.Session) {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "no sessions recorded")
		return
	}

	// Count by model
	models := make(map[string]int)
	fileCounts := make(map[string]int)
	totalTools := 0

	for _, s := range sessions {
		models[s.Agent.Model]++
		totalTools += len(s.ToolCalls)
		for _, c := range s.Commits {
			for _, f := range c.FilesChanged {
				fileCounts[f]++
			}
		}
	}

	fmt.Fprintf(w, "%s %s  %s  %s %s\n\n",
		Label("sessions:"), Number(len(sessions)),
		Separator("·"),
		Label("tool calls:"), Number(totalTools))

	fmt.Fprintln(w, Header("by model:"))
	for model, count := range models {
		fmt.Fprintf(w, "  %-25s %s\n", Model(model), Number(count))
	}
	fmt.Fprintln(w)

	if len(fileCounts) > 0 {
		fmt.Fprintln(w, Header("most touched files:"))
		for i := 0; i < 5 && i < len(fileCounts); i++ {
			maxFile := ""
			maxCount := 0
			for f, c := range fileCounts {
				if c > maxCount {
					maxFile = f
					maxCount = c
				}
			}
			if maxFile == "" {
				break
			}
			fmt.Fprintf(w, "  %-40s %s\n", FilePath(maxFile), Number(maxCount))
			delete(fileCounts, maxFile)
		}
	}
}

type GrepResult struct {
	Session *storage.Session
	Matches []string // matched lines/excerpts
}

func RenderSessionGrep(w io.Writer, query string, results []GrepResult) {
	for _, r := range results {
		fmt.Fprintf(w, "%s  %s  %s\n",
			SHA(truncID(r.Session.SessionID)),
			DateDim(r.Session.CreatedAt.Format("2006-01-02")),
			r.Session.Task.Prompt)
		for _, m := range r.Matches {
			fmt.Fprintf(w, "  %s\n", HighlightMatch(RenderMarkdown(m), query))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%s\n", HintText(fmt.Sprintf("%d sessions matched", len(results))))
}

func RenderSessionGrepJSON(w io.Writer, results []GrepResult) {
	data, _ := json.MarshalIndent(results, "", "  ")
	fmt.Fprintln(w, string(data))
}

func truncID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
