package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stevemurr/git-cognition/internal/output"
	"github.com/stevemurr/git-cognition/internal/retrieval"
	"github.com/stevemurr/git-cognition/internal/storage"
)

const defaultContext = 3 // lines above and below target

var (
	whyVerbose bool
	whyFull    bool
)

var whyCmd = &cobra.Command{
	Use:   "why <file>:<line>",
	Short: "Show why a line of code was written",
	Long:  "Traces a line back through git blame to the Claude Code session that produced it.",
	Args:  cobra.ExactArgs(1),
	RunE:  runWhy,
}

func init() {
	whyCmd.Flags().BoolVar(&whyVerbose, "verbose", false, "show full reasoning and action log")
	whyCmd.Flags().BoolVar(&whyFull, "full", false, "show everything including file contents read")
	rootCmd.AddCommand(whyCmd)
}

func runWhy(cmd *cobra.Command, args []string) error {
	fileLine := args[0]
	parts := strings.SplitN(fileLine, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected <file>:<line>, got %q", fileLine)
	}
	file := parts[0]
	line, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid line number: %q", parts[1])
	}

	// git blame
	sha, err := gitBlame(file, line)
	if err != nil {
		return fmt.Errorf("git blame failed: %w", err)
	}

	// Look up session note
	sessionID, err := storage.ReadSessionIDForCommit(sha)
	if err != nil {
		// No session note — fallback to plain blame output
		blameCmd := exec.Command("git", "blame", "-L", fmt.Sprintf("%d,%d", line, line), file)
		blameCmd.Stdout = os.Stdout
		blameCmd.Stderr = os.Stderr
		return blameCmd.Run()
	}

	// Load session
	session, err := storage.ReadSessionByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session %s: %w", sessionID, err)
	}

	// BM25 excerpt — only use if it's a quality match
	query := retrieval.QueryFromFilePath(file)
	excerpt := retrieval.BestExcerpt(query, session.Reasoning.FinalMessage)

	// Read code snippet around the target line
	codeLines, startLine := readCodeContext(file, line, defaultContext)

	data := output.WhyData{
		CommitSHA:  sha,
		FileLine:   fileLine,
		Session:    session,
		Excerpt:    excerpt,
		CodeLines:  codeLines,
		TargetLine: line,
		StartLine:  startLine,
	}

	w := os.Stdout
	switch {
	case jsonOutput:
		output.RenderWhyJSON(w, data)
	case whyFull:
		output.RenderWhyFull(w, data)
	case whyVerbose:
		output.RenderWhyVerbose(w, data)
	default:
		output.RenderWhyDefault(w, data)
	}
	return nil
}

func readCodeContext(file string, targetLine, context int) ([]string, int) {
	f, err := os.Open(file)
	if err != nil {
		return nil, 0
	}
	defer f.Close()

	startLine := targetLine - context
	if startLine < 1 {
		startLine = 1
	}
	endLine := targetLine + context

	var lines []string
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo < startLine {
			continue
		}
		if lineNo > endLine {
			break
		}
		lines = append(lines, scanner.Text())
	}
	return lines, startLine
}

func gitBlame(file string, line int) (string, error) {
	cmd := exec.Command("git", "blame", "-L", fmt.Sprintf("%d,%d", line, line), "--porcelain", file)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// First line of porcelain output: <sha> <orig-line> <final-line> <num-lines>
	firstLine := strings.SplitN(string(out), "\n", 2)[0]
	fields := strings.Fields(firstLine)
	if len(fields) == 0 {
		return "", fmt.Errorf("unexpected blame output")
	}
	return fields[0], nil
}
