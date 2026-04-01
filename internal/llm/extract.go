package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stevemurr/git-cognition/internal/storage"
)

var reasoningSchema = json.RawMessage(`{
  "type": "json_schema",
  "json_schema": {
    "name": "reasoning_extraction",
    "strict": true,
    "schema": {
      "type": "object",
      "properties": {
        "summary": {
          "type": "string",
          "description": "1-3 sentence summary of what was done and why"
        },
        "file_annotations": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "path": {"type": "string", "description": "relative file path"},
              "what": {"type": "string", "description": "what changed in this file"},
              "why": {"type": "string", "description": "why this change was needed"}
            },
            "required": ["path", "what", "why"],
            "additionalProperties": false
          }
        },
        "rejected_approaches": {
          "type": "array",
          "items": {"type": "string"},
          "description": "approaches considered but not chosen, with reasons"
        },
        "key_decisions": {
          "type": "array",
          "items": {"type": "string"},
          "description": "important design or implementation decisions"
        }
      },
      "required": ["summary", "file_annotations", "rejected_approaches", "key_decisions"],
      "additionalProperties": false
    }
  }
}`)

const systemPrompt = `You are a code reasoning extractor. Given a coding session transcript, produce a structured JSON summary. Be concise and precise. Focus on WHY decisions were made, not just WHAT was done. If no approaches were rejected, return an empty array.`

func ExtractReasoning(ctx context.Context, client *Client, session *storage.Session) (*storage.LLMReasoning, error) {
	userMsg := buildPrompt(session)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMsg},
	}

	content, err := client.Complete(ctx, messages, reasoningSchema)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	var result storage.LLMReasoning
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	// Ensure arrays are non-nil for clean JSON
	if result.FileAnnotations == nil {
		result.FileAnnotations = []storage.FileAnnotation{}
	}
	if result.RejectedApproaches == nil {
		result.RejectedApproaches = []string{}
	}
	if result.KeyDecisions == nil {
		result.KeyDecisions = []string{}
	}

	return &result, nil
}

func buildPrompt(session *storage.Session) string {
	var b strings.Builder

	// Task
	b.WriteString("## Task\n")
	b.WriteString(session.Task.Prompt)
	b.WriteString("\n\n")

	// Commits
	if len(session.Commits) > 0 {
		b.WriteString("## Commits\n")
		for _, c := range session.Commits {
			b.WriteString(fmt.Sprintf("- %s %s (files: %s)\n",
				c.SHA, c.Message, strings.Join(c.FilesChanged, ", ")))
		}
		b.WriteString("\n")
	}

	// Tool calls (abbreviated, max 30)
	if len(session.ToolCalls) > 0 {
		b.WriteString("## Tool Sequence\n")
		max := 30
		if len(session.ToolCalls) < max {
			max = len(session.ToolCalls)
		}
		for i := 0; i < max; i++ {
			tc := session.ToolCalls[i]
			b.WriteString(fmt.Sprintf("%d. %s\n", tc.Sequence, abbreviateToolCall(tc)))
		}
		if len(session.ToolCalls) > 30 {
			b.WriteString(fmt.Sprintf("+ %d more tool calls\n", len(session.ToolCalls)-30))
		}
		b.WriteString("\n")
	}

	// Final message (truncated)
	if session.Reasoning.FinalMessage != "" {
		b.WriteString("## Final Message\n")
		msg := session.Reasoning.FinalMessage
		if len(msg) > 3000 {
			msg = msg[:3000] + "\n[truncated]"
		}
		b.WriteString(msg)
		b.WriteString("\n\n")
	}

	b.WriteString("Extract the reasoning for this session.")

	return b.String()
}

func abbreviateToolCall(tc storage.ToolCall) string {
	var input map[string]interface{}
	json.Unmarshal(tc.Input, &input)

	switch tc.Tool {
	case "Read", "Edit", "Write":
		if fp, ok := input["file_path"].(string); ok {
			return fmt.Sprintf("%s %s", tc.Tool, fp)
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			if len(cmd) > 60 {
				cmd = cmd[:57] + "..."
			}
			return fmt.Sprintf("Bash: %s", cmd)
		}
	case "Glob":
		if p, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("Glob %s", p)
		}
	case "Grep":
		if p, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("Grep %s", p)
		}
	}

	return tc.Tool
}
