package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompleteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		resp := ChatResponse{
			Choices: []Choice{
				{Message: ChatMessage{Role: "assistant", Content: `{"summary":"test summary"}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "test-model", 5*time.Second)
	content, err := client.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "hello"},
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != `{"summary":"test summary"}` {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestCompleteHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model", 5*time.Second)
	_, err := client.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "hello"},
	}, nil)

	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestCompleteTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model", 100*time.Millisecond)
	_, err := client.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "hello"},
	}, nil)

	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestCompleteEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{Choices: []Choice{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model", 5*time.Second)
	_, err := client.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "hello"},
	}, nil)

	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestCompleteMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "model", 5*time.Second)
	_, err := client.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "hello"},
	}, nil)

	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
