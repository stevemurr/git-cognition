package retrieval

import (
	"testing"
)

func TestRankExcerptsSpecExample(t *testing.T) {
	document := `I've added a token bucket rate limiter to the auth endpoint.

I used the INCRBY/EXPIRE pattern already present in redis_client.py — this maps cleanly to token bucket semantics and means a single round trip per request with no new dependencies.

I considered two alternatives:
- A sliding window using ZADD/ZRANGEBYSCORE, but this requires a Lua script for atomicity and two round trips on what is already a hot path
- An in-process counter, but this wouldn't survive multiple workers or restarts

All existing tests pass. I added two new tests covering the rate limit boundary conditions.`

	query := QueryFromFilePath("auth/middleware.py")
	results := RankExcerpts(query, document)

	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}

	// The top result should mention "auth" since that's in our query
	found := false
	for _, r := range results[:1] {
		if contains(r.Text, "auth") || contains(r.Text, "token bucket") {
			found = true
		}
	}
	if !found {
		t.Errorf("top excerpt should be relevant to auth, got: %q", results[0].Text)
	}
}

func TestRankExcerptsTableDeprioritized(t *testing.T) {
	// Real-world scenario: claude -p produces a summary with a markdown table
	// that mentions every file. The table should not win over prose.
	document := "All 6 phases complete. Here's a summary:\n\n" +
		"| Phase | Commit | What |\n" +
		"|-------|--------|------|\n" +
		"| 1 | `d01575c` | `types.ts` (Todo interface) + `store.ts` (in-memory CRUD) |\n" +
		"| 2 | `6747b2b` | `cli.ts` (argument parser with flags support) |\n" +
		"| 3 | `1a567e3` | `commands.ts` + `main.ts` (add/list/complete/delete handlers) |\n" +
		"| 4 | `0b43c6c` | `persistence.ts` (JSON file load/save wired into main) |\n" +
		"| 5 | `cb61ead` | `.gitignore` + end-to-end verification of delete and filter |\n" +
		"| 6 | `a8ab013` | 19 tests across store, CLI parsing, and persistence |\n\n" +
		"Run with: `deno run --allow-read --allow-write main.ts <command>`\n" +
		"Test with: `deno test --allow-read --allow-write`"

	query := QueryFromFilePath("cli.ts")
	results := RankExcerpts(query, document)

	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// The top result should NOT be a table row
	top := results[0].Text
	if contains(top, "|") && contains(top, "Phase") {
		t.Errorf("top excerpt should not be a table row, got: %q", top)
	}
}

func TestRankExcerptsEmpty(t *testing.T) {
	results := RankExcerpts("test", "")
	if results != nil {
		t.Errorf("expected nil for empty document")
	}

	results = RankExcerpts("", "some text")
	if results != nil {
		t.Errorf("expected nil for empty query")
	}
}

func TestQueryFromFilePath(t *testing.T) {
	q := QueryFromFilePath("auth/middleware.py")
	if q != "auth middleware py" {
		t.Errorf("got %q", q)
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello, World! foo_bar")
	expected := []string{"hello", "world", "foo", "bar"}
	if len(tokens) != len(expected) {
		t.Fatalf("got %v, want %v", tokens, expected)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, expected[i])
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
