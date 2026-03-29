package capture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Claude Code JSONL transcript entry format:
// {"type": "user"|"assistant"|"system", "message": {"role": "...", "content": [...]}, ...}
type jsonlEntry struct {
	Type    string          `json:"type"`    // "user", "assistant", "system", etc.
	Message json.RawMessage `json:"message"` // API-format message object
}

type apiMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Model   string          `json:"model"`
}

// Spec-format transcript entry (JSON array format from spec):
// {"role": "user"|"assistant", "content": "..." | [...]}
type specEntry struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ExtractFinalMessage finds the last assistant message in the transcript
// and joins all text content blocks. Returns the verbatim text.
// Accepts Claude Code JSONL, spec JSON array, or spec JSONL.
func ExtractFinalMessage(data []byte) (string, error) {
	messages, err := parseTranscript(data)
	if err != nil {
		return "", err
	}

	// Walk backwards to find the last assistant entry
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].role != "assistant" {
			continue
		}
		return extractText(messages[i].content)
	}
	return "", nil
}

// ExtractTaskPrompt finds the first user message in the transcript.
func ExtractTaskPrompt(data []byte) (string, error) {
	messages, err := parseTranscript(data)
	if err != nil {
		return "", err
	}

	for _, m := range messages {
		if m.role != "user" {
			continue
		}
		return extractText(m.content)
	}
	return "", nil
}

type parsedMessage struct {
	role    string
	content json.RawMessage
	model   string
}

// parseTranscript handles three formats:
// 1. Claude Code JSONL: {"type":"user","message":{"role":"user","content":[...]}}
// 2. Spec JSON array: [{"role":"user","content":"..."},...]
// 3. Spec JSONL: {"role":"user","content":"..."}\n{...}
func parseTranscript(data []byte) ([]parsedMessage, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	// Try spec JSON array first
	if data[0] == '[' {
		var entries []specEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			var msgs []parsedMessage
			for _, e := range entries {
				if e.Role != "" {
					msgs = append(msgs, parsedMessage{role: e.Role, content: e.Content})
				}
			}
			return msgs, nil
		}
	}

	// Parse as JSONL
	var msgs []parsedMessage
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		// Try Claude Code JSONL format: {"type":"user","message":{...}}
		var ccEntry jsonlEntry
		if err := json.Unmarshal(line, &ccEntry); err == nil && ccEntry.Type != "" && len(ccEntry.Message) > 0 {
			role := ccEntry.Type
			if role == "user" || role == "assistant" {
				var msg apiMessage
				if err := json.Unmarshal(ccEntry.Message, &msg); err == nil && len(msg.Content) > 0 {
					msgs = append(msgs, parsedMessage{role: role, content: msg.Content, model: msg.Model})
					continue
				}
			}
		}

		// Try spec JSONL format: {"role":"user","content":"..."}
		var specE specEntry
		if err := json.Unmarshal(line, &specE); err == nil && specE.Role != "" {
			msgs = append(msgs, parsedMessage{role: specE.Role, content: specE.Content})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("transcript: scan JSONL: %w", err)
	}
	return msgs, nil
}

// ExtractModel finds the model from the first assistant message in the transcript.
func ExtractModel(data []byte) string {
	messages, err := parseTranscript(data)
	if err != nil {
		return ""
	}
	for _, m := range messages {
		if m.role == "assistant" && m.model != "" {
			return m.model
		}
	}
	return ""
}

func extractText(content json.RawMessage) (string, error) {
	if len(content) == 0 {
		return "", nil
	}

	// Try as string first (simple content)
	var s string
	if err := json.Unmarshal(content, &s); err == nil {
		return s, nil
	}

	// Try as array of content blocks
	var blocks []contentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return "", fmt.Errorf("transcript: unmarshal content blocks: %w", err)
	}

	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}
