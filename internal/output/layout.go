package output

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// TermWidth returns the current terminal width, or 120 as fallback.
func TermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		return 120
	}
	return w
}

// IsTTY reports whether stdout is a terminal.
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// lipgloss styles for the two-pane layout
var (
	leftPaneStyle = lipgloss.NewStyle().
			PaddingRight(1).
			PaddingLeft(1)

	rightPaneStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1)

	// Section headers — green with a horizontal rule feel
	richHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10")) // bright green

	// Pane title bars — inverted green for the top of each pane
	paneTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("10")).
			PaddingLeft(1).
			PaddingRight(1)

	// Divider column
	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	// Tool names in the sequence list
	richToolName = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).  // bright cyan
			Bold(true)

	// File paths in tool calls
	richFilePath = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")) // white

	// Bash commands
	richBashCmd = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")) // yellow

	// Sequence numbers
	richSeqNum = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")). // dim gray
			Bold(false)

	// Task prompt text
	richTaskText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")). // bright white
			Italic(true)

	// Reasoning text
	richReasonText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")) // white

	// Rejected approach bullets
	richRejected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")) // bright red

	// Commit SHA
	richSHA = lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")). // yellow
		Bold(true)

	// Model name
	richModel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")) // cyan

	// Date
	richDate = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // dim

	// Dot separator
	richDot = lipgloss.NewStyle().
		Foreground(lipgloss.Color("238"))

	// Target line highlight in code
	richTargetLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")). // bright white
			Bold(true)

	// Target line number
	richTargetNum = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")). // yellow
			Bold(true)

	// Context line number
	richLineNum = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238")) // very dim

	// Context code lines
	richCodeDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // gray

	// Arrow marker for target line
	richArrow = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // green
			Bold(true)

	// Horizontal rule characters
	richRule = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))
)

// wrapText wraps text to the given width, preserving existing newlines.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	for i, paragraph := range strings.Split(text, "\n") {
		if i > 0 {
			result.WriteByte('\n')
		}
		if len(paragraph) <= width {
			result.WriteString(paragraph)
			continue
		}
		// Word wrap this line
		words := strings.Fields(paragraph)
		lineLen := 0
		for j, word := range words {
			wl := len(word)
			if j > 0 && lineLen+1+wl > width {
				result.WriteByte('\n')
				lineLen = 0
			} else if j > 0 {
				result.WriteByte(' ')
				lineLen++
			}
			result.WriteString(word)
			lineLen += wl
		}
	}
	return result.String()
}

// truncateLines limits a string to maxLines lines, appending "..." if truncated.
func truncateLines(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

// verticalDivider returns a string of vertical bar characters for the given height.
func verticalDivider(height int) string {
	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("│")
	}
	return dividerStyle.Render(b.String())
}
