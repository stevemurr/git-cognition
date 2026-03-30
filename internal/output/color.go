package output

import (
	"regexp"
	"strings"

	"github.com/fatih/color"
)

var (
	SHA       = color.New(color.FgYellow).SprintFunc()
	FilePath  = color.New(color.FgWhite, color.Bold).SprintFunc()
	Model     = color.New(color.FgCyan).SprintFunc()
	DateDim   = color.New(color.Faint).SprintFunc()
	Quote     = color.New(color.FgGreen).SprintFunc()
	Header    = color.New(color.Bold).SprintFunc()
	HeaderDim = color.New(color.Bold, color.Faint).SprintFunc()
	ToolName  = color.New(color.FgCyan).SprintFunc()
	Number    = color.New(color.Bold).SprintFunc()
	Label     = color.New(color.Faint).SprintFunc()
	Separator = color.New(color.Faint).SprintFunc()
	MatchHi   = color.New(color.FgYellow, color.Bold).SprintFunc()
	HintText  = color.New(color.Faint).SprintFunc()
)

var (
	InlineCode = color.New(color.FgCyan).SprintFunc()
	Bold       = color.New(color.Bold).SprintFunc()
	Bullet     = color.New(color.Faint).SprintFunc()
)

var (
	reBold       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reInlineCode = regexp.MustCompile("`([^`]+)`")
)

// RenderMarkdown converts a subset of markdown to ANSI-styled text:
// **bold**, `code`, and bullet prefixes.
func RenderMarkdown(text string) string {
	// Bold: **text** → ANSI bold
	text = reBold.ReplaceAllStringFunc(text, func(m string) string {
		inner := m[2 : len(m)-2]
		// Handle nested backticks inside bold: **`code`**
		inner = renderInlineCode(inner)
		return Bold(inner)
	})

	// Inline code: `text` → cyan
	text = renderInlineCode(text)

	// Bullet prefix
	if strings.HasPrefix(text, "- ") {
		text = Bullet("  · ") + text[2:]
	}

	return text
}

func renderInlineCode(text string) string {
	return reInlineCode.ReplaceAllStringFunc(text, func(m string) string {
		return InlineCode(m[1 : len(m)-1])
	})
}

func HighlightMatch(text, query string) string {
	lower := strings.ToLower(text)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	if idx == -1 {
		return text
	}
	return text[:idx] + MatchHi(text[idx:idx+len(query)]) + text[idx+len(query):]
}
