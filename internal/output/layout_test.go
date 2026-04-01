package output

import (
	"testing"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{"short line", "hello world", 80, "hello world"},
		{"long line", "the quick brown fox jumps over the lazy dog", 20, "the quick brown fox\njumps over the lazy\ndog"},
		{"preserves newlines", "line one\nline two", 80, "line one\nline two"},
		{"zero width", "hello", 0, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.text, tt.width)
			if got != tt.want {
				t.Errorf("wrapText(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}

func TestTruncateLines(t *testing.T) {
	text := "line1\nline2\nline3\nline4\nline5"
	got := truncateLines(text, 3)
	if got != "line1\nline2\nline3\n..." {
		t.Errorf("truncateLines(5 lines, 3) = %q", got)
	}

	// No truncation needed
	got2 := truncateLines("a\nb", 5)
	if got2 != "a\nb" {
		t.Errorf("truncateLines(2 lines, 5) = %q", got2)
	}
}
