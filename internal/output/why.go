package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/stevemurr/git-cognition/internal/storage"
)

type WhyData struct {
	CommitSHA  string
	FileLine   string
	Session    *storage.Session
	Excerpt    string   // BM25-selected excerpt from final_message
	CodeLines  []string // lines of code around the target
	TargetLine int      // 1-based line number of the target
	StartLine  int      // 1-based line number of the first line in CodeLines
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

	fmt.Fprintln(w, Header("claude's reasoning:"))
	if d.Session.Reasoning.FinalMessage != "" {
		for _, line := range strings.Split(d.Session.Reasoning.FinalMessage, "\n") {
			fmt.Fprintf(w, "  %s\n", Quote("\""+RenderMarkdown(line)))
		}
	}
	fmt.Fprintln(w)

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
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(w, string(data))
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
