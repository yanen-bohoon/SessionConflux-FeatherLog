package parser

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ExtractMeta reads a JSONL file and returns session metadata.
// Reads the first user message for title, counts total lines.
func ExtractMeta(filePath string) (*SessionMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sessionID := sessionIDFromPath(filePath)
	meta := &SessionMeta{
		SessionID: sessionID,
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max line
	for scanner.Scan() {
		meta.MessageCount++
		line := scanner.Bytes()
		if meta.Title == "" {
			meta.Title = extractTitle(line)
		}
	}
	if meta.Title == "" {
		meta.Title = sessionID
	}
	return meta, scanner.Err()
}

func sessionIDFromPath(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func extractTitle(line []byte) string {
	var msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &msg) != nil {
		return ""
	}

	// Claude Code format: {"role": "user", "content": [{"type": "text", "text": "..."}]}
	if msg.Role == "user" {
		content := msg.Content
		if content == "" {
			// Try message.content (nested format)
			if msg.Message.Content != nil {
				raw := msg.Message.Content
				// Could be string or array
				if raw[0] == '"' {
					json.Unmarshal(raw, &content)
				} else if raw[0] == '[' {
					var parts []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					}
					if json.Unmarshal(raw, &parts) == nil {
						for _, p := range parts {
							if p.Type == "text" {
								content = p.Text
								break
							}
						}
					}
				}
			}
		}
		if content != "" {
			return truncateString(content, 80)
		}
	}
	return ""
}

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
