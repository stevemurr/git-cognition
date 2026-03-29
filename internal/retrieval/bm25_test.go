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
