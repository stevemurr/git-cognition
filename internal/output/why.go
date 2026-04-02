package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"github.com/stevemurr/git-cognition/internal/storage"
)

type WhyData struct {
	CommitSHA    string
	FileLine     string
	Session      *storage.Session
	Excerpt      string                 // LLM excerpt or task prompt fallback
	LLMReasoning *storage.LLMReasoning  // nil for old sessions
	CodeLines    []string               // lines of code around the target
	TargetLine   int                    // 1-based line number of the target
	StartLine    int                    // 1-based line number of the first line in CodeLines
}

var targetLineHL = color.New(color.FgWhite, color.Bold).SprintFunc()
var lineNumHL = color.New(color.FgYellow, color.Bold).SprintFunc()
var lineNum = color.New(color.Faint).SprintFunc()
var codeDim = color.New(color.Faint).SprintFunc()

func renderCodeSnippet(w io.Writer, d WhyData) {
	if len(d.CodeLines) == 0 {
		return
	}
	// Find the widest line number for alignment
	maxLineNo := d.StartLine + len(d.CodeLines) - 1
	width := len(fmt.Sprintf("%d", maxLineNo))

	for i, code := range d.CodeLines {
		lineNo := d.StartLine + i
		if lineNo == d.TargetLine {
			marker := "→"
			fmt.Fprintf(w, "  %s %s %s\n",
				lineNumHL(fmt.Sprintf("%*d", width, lineNo)),
				marker,
				targetLineHL(code))
		} else {
			fmt.Fprintf(w, "  %s   %s\n",
				lineNum(fmt.Sprintf("%*d", width, lineNo)),
				codeDim(code))
		}
	}
	fmt.Fprintln(w)
}

func RenderWhyDefault(w io.Writer, d WhyData) {
	fmt.Fprintf(w, "%s  %s  %s  %s  %s  %s\n",
		SHA(d.CommitSHA), FilePath(d.FileLine),
		Separator("·"), Model(d.Session.Agent.Model),
		Separator("·"), DateDim(d.Session.CreatedAt.Format("2006-01-02")))
	fmt.Fprintln(w)

	renderCodeSnippet(w, d)

	if d.Excerpt != "" {
		for _, line := range strings.Split(d.Excerpt, "\n") {
			fmt.Fprintf(w, "  %s\n", Quote("\""+RenderMarkdown(line)))
		}
	} else if d.Session.Task.Prompt != "" {
		fmt.Fprintf(w, "  %s %s\n", Label("task:"), d.Session.Task.Prompt)
	} else if d.Session.Reasoning.FinalMessage != "" {
		lines := strings.SplitN(d.Session.Reasoning.FinalMessage, "\n", 3)
		for _, line := range lines[:min(len(lines), 2)] {
			fmt.Fprintf(w, "  %s\n", Quote("\""+RenderMarkdown(line)))
		}
	}
}

func RenderWhyVerbose(w io.Writer, d WhyData) {
	fmt.Fprintf(w, "%s  %s  %s  %s  %s  %s\n",
		SHA(d.CommitSHA), FilePath(d.FileLine),
		Separator("·"), Model(d.Session.Agent.Model),
		Separator("·"), DateDim(d.Session.CreatedAt.Format("2006-01-02")))
	fmt.Fprintf(w, "%s %s  %s  %s %s\n",
		Label("session:"), SHA(d.Session.SessionID),
		Separator("·"),
		Label("task:"), d.Session.Task.Prompt)
	fmt.Fprintln(w)

	renderCodeSnippet(w, d)

	if d.LLMReasoning != nil {
		fmt.Fprintln(w, Header("reasoning:"))
		fmt.Fprintf(w, "  %s\n", Quote("\""+d.LLMReasoning.Summary))
		fmt.Fprintln(w)

		if len(d.LLMReasoning.KeyDecisions) > 0 {
			fmt.Fprintln(w, Header("key decisions:"))
			for _, dec := range d.LLMReasoning.KeyDecisions {
				fmt.Fprintf(w, "  %s %s\n", Separator("·"), dec)
			}
			fmt.Fprintln(w)
		}

		if len(d.LLMReasoning.RejectedApproaches) > 0 {
			fmt.Fprintln(w, Header("rejected:"))
			for _, r := range d.LLMReasoning.RejectedApproaches {
				fmt.Fprintf(w, "  %s %s\n", Separator("·"), r)
			}
			fmt.Fprintln(w)
		}
	}

	if d.Session.Reasoning.FinalMessage != "" {
		fmt.Fprintln(w, Header("claude's message:"))
		for _, line := range strings.Split(d.Session.Reasoning.FinalMessage, "\n") {
			fmt.Fprintf(w, "  %s\n", Quote("\""+RenderMarkdown(line)))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, Header("action log:"))
	for _, tc := range d.Session.ToolCalls {
		desc := formatToolCallShort(tc)
		fmt.Fprintf(w, "  %s\n", desc)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n", HintText(fmt.Sprintf("git why %s --full  ·  git session show %s", d.FileLine, d.Session.SessionID)))
}

func RenderWhyFull(w io.Writer, d WhyData) {
	RenderWhyVerbose(w, d)
	fmt.Fprintln(w)

	fmt.Fprintln(w, Header("files read during session:"))
	for _, tc := range d.Session.ToolCalls {
		if tc.Tool != "Read" {
			continue
		}
		var input struct {
			FilePath string `json:"file_path"`
		}
		json.Unmarshal(tc.Input, &input)
		if input.FilePath == "" {
			continue
		}
		fmt.Fprintf(w, "\n  %s\n", FilePath(input.FilePath+":"))
		for _, line := range strings.Split(tc.OutputTruncated, "\n") {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}
}

func RenderWhyJSON(w io.Writer, d WhyData) {
	out := map[string]interface{}{
		"commit_sha":  d.CommitSHA,
		"file_line":   d.FileLine,
		"session":     d.Session,
		"excerpt":     d.Excerpt,
		"code_lines":  d.CodeLines,
		"target_line": d.TargetLine,
		"start_line":  d.StartLine,
	}
	if d.LLMReasoning != nil {
		out["llm_reasoning"] = d.LLMReasoning
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(w, string(data))
}

// RenderWhyRich renders a two-pane layout: code on left, context on right.
func RenderWhyRich(w io.Writer, d WhyData, termWidth int) {
	if termWidth < 80 {
		fmt.Fprintln(w, HintText("terminal too narrow for rich view, showing verbose output"))
		RenderWhyVerbose(w, d)
		return
	}

	dividerWidth := 3 // " │ "
	leftWidth := (termWidth * 40) / 100
	rightWidth := termWidth - leftWidth - dividerWidth
	innerRight := rightWidth - 4 // account for pane padding

	// --- Left pane: code snippet ---
	var left strings.Builder

	// Title bar
	left.WriteString(paneTitleStyle.Render(fmt.Sprintf(" %s ", d.FileLine)))
	left.WriteByte('\n')
	left.WriteString(richRule.Render(strings.Repeat("─", leftWidth-2)))
	left.WriteByte('\n')

	if len(d.CodeLines) > 0 {
		// Extract filename for syntax highlighting
		fileName := d.FileLine
		if idx := strings.LastIndex(fileName, ":"); idx > 0 {
			fileName = fileName[:idx]
		}
		highlighted := highlightCode(fileName, d.CodeLines)

		maxLineNo := d.StartLine + len(d.CodeLines) - 1
		numW := len(fmt.Sprintf("%d", maxLineNo))
		for i, code := range highlighted {
			lineNo := d.StartLine + i
			if lineNo == d.TargetLine {
				left.WriteString(fmt.Sprintf("  %s %s %s\n",
					richTargetNum.Render(fmt.Sprintf("%*d", numW, lineNo)),
					richArrow.Render("→"),
					code))
			} else {
				left.WriteString(fmt.Sprintf("  %s   %s\n",
					richLineNum.Render(fmt.Sprintf("%*d", numW, lineNo)),
					code))
			}
		}
	}

	leftPane := leftPaneStyle.Width(leftWidth).Render(left.String())

	// --- Right pane: context ---
	var right strings.Builder

	// Title bar
	right.WriteString(paneTitleStyle.Render(" Context "))
	right.WriteByte('\n')
	right.WriteString(richRule.Render(strings.Repeat("─", rightWidth-2)))
	right.WriteByte('\n')

	// Commit info
	right.WriteString(fmt.Sprintf("%s  %s  %s  %s  %s\n",
		richSHA.Render(d.CommitSHA[:10]),
		richDot.Render("·"),
		richModel.Render(d.Session.Agent.Model),
		richDot.Render("·"),
		richDate.Render(d.Session.CreatedAt.Format("2006-01-02 15:04"))))
	right.WriteByte('\n')

	// Task
	right.WriteString(richHeader.Render("Task"))
	right.WriteByte('\n')
	task := wrapText(d.Session.Task.Prompt, innerRight)
	for _, line := range strings.Split(task, "\n") {
		right.WriteString(fmt.Sprintf("  %s\n", richTaskText.Render(line)))
	}
	right.WriteByte('\n')

	// Tool sequence
	if len(d.Session.ToolCalls) > 0 {
		right.WriteString(richHeader.Render("Tool Sequence"))
		right.WriteByte('\n')
		maxShow := 12
		for i, tc := range d.Session.ToolCalls {
			if i >= maxShow {
				right.WriteString(fmt.Sprintf("  %s\n",
					richDate.Render(fmt.Sprintf("+ %d more", len(d.Session.ToolCalls)-maxShow))))
				break
			}
			right.WriteString(fmt.Sprintf("  %s %s\n",
				richSeqNum.Render(fmt.Sprintf("%2d.", tc.Sequence)),
				formatToolCallRich(tc)))
		}
		right.WriteByte('\n')
	}

	// Reasoning
	if d.LLMReasoning != nil {
		right.WriteString(richHeader.Render("Reasoning"))
		right.WriteByte('\n')
		summary := wrapText(d.LLMReasoning.Summary, innerRight)
		for _, line := range strings.Split(summary, "\n") {
			right.WriteString(fmt.Sprintf("  %s\n", richReasonText.Render(line)))
		}
		right.WriteByte('\n')

		// Key decisions
		if len(d.LLMReasoning.KeyDecisions) > 0 {
			right.WriteString(richHeader.Render("Key Decisions"))
			right.WriteByte('\n')
			for _, dec := range d.LLMReasoning.KeyDecisions {
				wrapped := wrapText(dec, innerRight-4)
				lines := strings.Split(wrapped, "\n")
				for j, line := range lines {
					if j == 0 {
						right.WriteString(fmt.Sprintf("  %s %s\n",
							richDot.Render("·"), richReasonText.Render(line)))
					} else {
						right.WriteString(fmt.Sprintf("    %s\n", richReasonText.Render(line)))
					}
				}
			}
			right.WriteByte('\n')
		}

		// Rejected approaches (structured)
		if len(d.LLMReasoning.RejectedApproaches) > 0 {
			right.WriteString(richRejected.Render("Rejected"))
			right.WriteByte('\n')
			for _, r := range d.LLMReasoning.RejectedApproaches {
				wrapped := wrapText(r, innerRight-4)
				lines := strings.Split(wrapped, "\n")
				for j, line := range lines {
					if j == 0 {
						right.WriteString(fmt.Sprintf("  %s %s\n",
							richRejected.Render("·"), richDate.Render(line)))
					} else {
						right.WriteString(fmt.Sprintf("    %s\n", richDate.Render(line)))
					}
				}
			}
		}
	}

	// Claude's message — always shown when available
	if d.Session.Reasoning.FinalMessage != "" {
		right.WriteString(richHeader.Render("Claude's Message"))
		right.WriteByte('\n')
		msg := wrapText(d.Session.Reasoning.FinalMessage, innerRight)
		msg = truncateLines(msg, 10)
		for _, line := range strings.Split(msg, "\n") {
			right.WriteString(fmt.Sprintf("  %s\n", richReasonText.Render(RenderMarkdown(line))))
		}
		right.WriteByte('\n')
	}

	rightPane := rightPaneStyle.Width(rightWidth).Render(right.String())

	// Divider
	leftH := strings.Count(leftPane, "\n") + 1
	rightH := strings.Count(rightPane, "\n") + 1
	maxH := leftH
	if rightH > maxH {
		maxH = rightH
	}
	divider := verticalDivider(maxH)

	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, divider, rightPane)
	fmt.Fprintln(w, joined)
}

// formatToolCallRich returns a colorized short description for tool calls
// in the rich two-pane view.
func formatToolCallRich(tc storage.ToolCall) string {
	var input map[string]interface{}
	json.Unmarshal(tc.Input, &input)

	tool := richToolName.Render(fmt.Sprintf("%-5s", tc.Tool))

	switch tc.Tool {
	case "Read":
		if fp, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("%s %s", tool, richFilePath.Render(fp))
		}
	case "Edit", "Write":
		if fp, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("%s %s", tool, richFilePath.Render(fp))
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			if len(cmd) > 40 {
				cmd = cmd[:37] + "..."
			}
			return fmt.Sprintf("%s %s", tool, richBashCmd.Render(cmd))
		}
	case "Glob":
		if p, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("%s %s", tool, richFilePath.Render(p))
		}
	case "Grep":
		if p, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("%s %s", tool, richBashCmd.Render(p))
		}
	}

	return fmt.Sprintf("%s %s", tool, string(tc.Input))
}

func formatToolCallShort(tc storage.ToolCall) string {
	var input map[string]interface{}
	json.Unmarshal(tc.Input, &input)

	switch tc.Tool {
	case "Read":
		if fp, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("%s  %s", ToolName("Read"), FilePath(fp))
		}
	case "Edit", "Write":
		if fp, ok := input["file_path"].(string); ok {
			desc := ""
			if d, ok := input["description"].(string); ok {
				desc = fmt.Sprintf(" — %q", d)
			}
			return fmt.Sprintf("%s  %s%s", ToolName(tc.Tool), FilePath(fp), desc)
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			out := tc.OutputTruncated
			if len(out) > 60 {
				out = out[:60] + "..."
			}
			if out != "" {
				return fmt.Sprintf("%s  %s  %s  %s", ToolName("Bash"), cmd, Separator("→"), DateDim(out))
			}
			return fmt.Sprintf("%s  %s", ToolName("Bash"), cmd)
		}
	}

	return fmt.Sprintf("%s  %v", ToolName(tc.Tool), string(tc.Input))
}
