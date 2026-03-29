package capture

import (
	"testing"
)

func TestExtractFinalMessage(t *testing.T) {
	transcript := []byte(`[
		{"role": "user", "content": "Add rate limiting to the auth endpoint"},
		{
			"role": "assistant",
			"content": [
				{"type": "text", "text": "I'll start by reading the existing middleware..."},
				{"type": "tool_use", "name": "Read", "input": {"file_path": "redis_client.py"}}
			]
		},
		{
			"role": "assistant",
			"content": [
				{
					"type": "text",
					"text": "I've added a token bucket rate limiter to the auth endpoint."
				}
			]
		}
	]`)

	msg, err := ExtractFinalMessage(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "I've added a token bucket rate limiter to the auth endpoint." {
		t.Errorf("got %q", msg)
	}
}

func TestExtractFinalMessageJSONL(t *testing.T) {
	transcript := []byte(`{"role": "user", "content": "Fix the bug"}
{"role": "assistant", "content": [{"type": "text", "text": "I'll look into it."}]}
{"role": "assistant", "content": [{"type": "text", "text": "I fixed the bug by updating the handler."}]}
`)

	msg, err := ExtractFinalMessage(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "I fixed the bug by updating the handler." {
		t.Errorf("got %q", msg)
	}
}

func TestExtractFinalMessageMultipleTextBlocks(t *testing.T) {
	transcript := []byte(`[
		{"role": "assistant", "content": [
			{"type": "text", "text": "Part one."},
			{"type": "text", "text": "Part two."}
		]}
	]`)

	msg, err := ExtractFinalMessage(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "Part one.\nPart two." {
		t.Errorf("got %q", msg)
	}
}

func TestExtractFinalMessageEmpty(t *testing.T) {
	msg, err := ExtractFinalMessage([]byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	if msg != "" {
		t.Errorf("expected empty, got %q", msg)
	}
}

func TestExtractTaskPrompt(t *testing.T) {
	transcript := []byte(`[
		{"role": "user", "content": "Fix the login bug"},
		{"role": "assistant", "content": [{"type": "text", "text": "On it."}]}
	]`)

	prompt, err := ExtractTaskPrompt(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "Fix the login bug" {
		t.Errorf("got %q", prompt)
	}
}

func TestExtractTaskPromptJSONL(t *testing.T) {
	transcript := []byte(`{"role": "user", "content": "Fix the login bug"}
{"role": "assistant", "content": [{"type": "text", "text": "On it."}]}
`)

	prompt, err := ExtractTaskPrompt(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "Fix the login bug" {
		t.Errorf("got %q", prompt)
	}
}

func TestExtractTaskPromptWithContentBlocks(t *testing.T) {
	transcript := []byte(`[
		{"role": "user", "content": [{"type": "text", "text": "Fix it"}]}
	]`)

	prompt, err := ExtractTaskPrompt(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "Fix it" {
		t.Errorf("got %q", prompt)
	}
}
