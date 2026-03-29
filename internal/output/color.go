package output

import (
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

func HighlightMatch(text, query string) string {
	lower := strings.ToLower(text)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	if idx == -1 {
		return text
	}
	return text[:idx] + MatchHi(text[idx:idx+len(query)]) + text[idx+len(query):]
}
