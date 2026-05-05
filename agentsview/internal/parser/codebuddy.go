package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// ParseCodeBuddySession parses a CodeBuddy JSONL session file.
func ParseCodeBuddySession(path, project, machine string) (*ParsedSession, []ParsedMessage, error) {
	return parseCodeBuddySession(path, project, machine, AgentCodeBuddy)
}

// ParseWorkBuddySession parses a WorkBuddy JSONL session file.
func ParseWorkBuddySession(path, project, machine string) (*ParsedSession, []ParsedMessage, error) {
	return parseCodeBuddySession(path, project, machine, AgentWorkBuddy)
}

// parseCodeBuddySession parses a CodeBuddy/WorkBuddy JSONL session file.
//
// Directory layout:
// ~/.codebuddy/projects/{project-name}/{uuid}.jsonl
// ~/.codebuddy/projects/{project-name}/{uuid}/subagents/agent-{subagentId}.jsonl
//
// WorkBuddy uses ~/.workbuddy/projects/ with the same layout.
func parseCodeBuddySession(path, project, machine string, agent AgentType) (*ParsedSession, []ParsedMessage, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("stat %s: %w", path, err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	lr := newLineReader(f, maxLineSize)

	var (
		messages         []ParsedMessage
		startedAt        time.Time
		endedAt          time.Time
		cwd              string
		sessionId        string
		ordinal          int
		userMessageCount int
		firstMsg         string
	)

	for {
		line, ok := lr.next()
		if !ok {
			break
		}
		if !gjson.Valid(line) {
			continue
		}

		res := gjson.Parse(line)
		lineType := res.Get("type").Str

		// Extract session metadata from any line that has it
		if sessionId == "" {
			sessionId = res.Get("sessionId").Str
		}
		if cwd == "" {
			cwd = res.Get("cwd").Str
		}

		tsMilli := res.Get("timestamp").Int()
		ts := time.UnixMilli(tsMilli)

		if !ts.IsZero() {
			if startedAt.IsZero() || ts.Before(startedAt) {
				startedAt = ts
			}
			if ts.After(endedAt) {
				endedAt = ts
			}
		}

		switch lineType {
		case "message":
			role := res.Get("role").Str
			content := res.Get("content")

			// Use shared ExtractTextContent helper. Note: toolCalls/toolResults
			// are ignored here as CodeBuddy uses independent line types for them.
			text, thinking, hasThinking, _, _, _ := ExtractTextContent(content)

			msg := ParsedMessage{
				Ordinal:      ordinal,
				Role:         RoleType(role),
				Content:      text,
				ThinkingText: thinking,
				HasThinking:  hasThinking,
				Timestamp:    ts,
			}
			messages = append(messages, msg)
			ordinal++

			if role == string(RoleUser) {
				userMessageCount++
				if firstMsg == "" {
					firstMsg = truncate(text, 200)
				}
			}

		case "function_call":
			callId := res.Get("callId").Str
			name := res.Get("name").Str
			args := res.Get("arguments").Str

			msg := ParsedMessage{
				Ordinal:    ordinal,
				Role:       RoleAssistant,
				HasToolUse: true,
				Timestamp:  ts,
				ToolCalls: []ParsedToolCall{
					{
						ToolUseID: callId,
						ToolName:  name,
						Category:  NormalizeToolCategory(name),
						InputJSON: args,
					},
				},
			}

			// Token extraction (in function_call line)
			// Priority: providerData.rawUsage -> providerData.usage -> message.usage
			usage := res.Get("providerData.rawUsage")
			if !usage.Exists() {
				usage = res.Get("providerData.usage")
			}
			if !usage.Exists() {
				usage = res.Get("message.usage")
			}

			if usage.Exists() {
				msg.TokenUsage = []byte(usage.Raw)

				// Extract context/output tokens for internal fields
				if usage.Get("prompt_tokens").Exists() {
					msg.ContextTokens = int(usage.Get("prompt_tokens").Int() +
						usage.Get("cache_creation_input_tokens").Int() +
						usage.Get("cache_read_input_tokens").Int())
					msg.OutputTokens = int(usage.Get("completion_tokens").Int())
					msg.HasContextTokens = true
					msg.HasOutputTokens = true
				} else if usage.Get("inputTokens").Exists() {
					msg.ContextTokens = int(usage.Get("inputTokens").Int())
					msg.OutputTokens = int(usage.Get("outputTokens").Int())
					msg.HasContextTokens = true
					msg.HasOutputTokens = true
				} else if usage.Get("input_tokens").Exists() {
					msg.ContextTokens = int(usage.Get("input_tokens").Int())
					msg.OutputTokens = int(usage.Get("output_tokens").Int())
					msg.HasContextTokens = true
					msg.HasOutputTokens = true
				}
				msg.tokenPresenceKnown = true
			}

			messages = append(messages, msg)
			ordinal++

		case "function_call_result":
			callId := res.Get("callId").Str
			// output: {type: "text", text: "..."}
			output := res.Get("output.text").Str

			// contentRaw should be JSON string of the text result
			contentRaw, _ := json.Marshal(output)

			msg := ParsedMessage{
				Ordinal:   ordinal,
				Role:      RoleUser,
				Timestamp: ts,
				ToolResults: []ParsedToolResult{
					{
						ToolUseID:  callId,
						ContentRaw: string(contentRaw),
					},
				},
			}
			messages = append(messages, msg)
			ordinal++

		case "file-history-snapshot":
			continue
		}
	}

	if sessionId == "" {
		sessionId = CodeBuddySessionID(filepath.Base(path))
	}

	var parentSessionId string
	// Check if this is a subagent session by looking at the path
	// ~/.codebuddy/projects/{project-name}/{parent-uuid}/subagents/agent-{subagentId}.jsonl
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "subagents" {
		parentDir := filepath.Dir(dir)
		parentSessionId = string(agent) + ":" + filepath.Base(parentDir)
	}

	if project == "" {
		project = ExtractProjectFromCwd(cwd)
	}
	if project == "" {
		project = "unknown"
	}

	sess := &ParsedSession{
		ID:               string(agent) + ":" + sessionId,
		Project:          project,
		Machine:          machine,
		Agent:            agent,
		ParentSessionID:  parentSessionId,
		Cwd:              cwd,
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		FirstMessage:     firstMsg,
		MessageCount:     len(messages),
		UserMessageCount: userMessageCount,
		File: FileInfo{
			Path:  path,
			Size:  info.Size(),
			Mtime: info.ModTime().UnixNano(),
		},
	}

	return sess, messages, nil
}

// DiscoverCodeBuddyProjects finds CodeBuddy session files.
func DiscoverCodeBuddyProjects(projectsDir string) []DiscoveredFile {
	return discoverCodeBuddyProjects(projectsDir, AgentCodeBuddy)
}

// DiscoverWorkBuddyProjects finds WorkBuddy session files.
func DiscoverWorkBuddyProjects(projectsDir string) []DiscoveredFile {
	return discoverCodeBuddyProjects(projectsDir, AgentWorkBuddy)
}

func discoverCodeBuddyProjects(projectsDir string, agent AgentType) []DiscoveredFile {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}

	var files []DiscoveredFile
	for _, entry := range entries {
		if !isDirOrSymlink(entry, projectsDir) {
			continue
		}

		projDir := filepath.Join(projectsDir, entry.Name())
		sessionFiles, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}

		for _, sf := range sessionFiles {
			if sf.IsDir() {
				continue
			}
			name := sf.Name()
			if !strings.HasSuffix(name, ".jsonl") {
				continue
			}
			stem := strings.TrimSuffix(name, ".jsonl")
			if strings.HasPrefix(stem, "agent-") {
				continue
			}
			files = append(files, DiscoveredFile{
				Path:    filepath.Join(projDir, name),
				Project: entry.Name(),
				Agent:   agent,
			})
		}

		// Scan session directories for subagent files
		for _, sf := range sessionFiles {
			if !sf.IsDir() {
				continue
			}
			subagentsDir := filepath.Join(
				projDir, sf.Name(), "subagents",
			)
			subFiles, err := os.ReadDir(subagentsDir)
			if err != nil {
				continue
			}
			for _, sub := range subFiles {
				if sub.IsDir() {
					continue
				}
				name := sub.Name()
				if !strings.HasPrefix(name, "agent-") ||
					!strings.HasSuffix(name, ".jsonl") {
					continue
				}
				files = append(files, DiscoveredFile{
					Path: filepath.Join(
						subagentsDir, name,
					),
					Project: entry.Name(),
					Agent:   agent,
				})
			}
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files
}

// FindCodeBuddySourceFile locates a CodeBuddy/WorkBuddy session file.
func FindCodeBuddySourceFile(projectsDir, sessionID string) string {
	// Not using IsValidSessionID because it might be too restrictive for UUIDs
	if sessionID == "" {
		return ""
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}

	target := sessionID + ".jsonl"
	for _, entry := range entries {
		if !isDirOrSymlink(entry, projectsDir) {
			continue
		}
		candidate := filepath.Join(
			projectsDir, entry.Name(), target,
		)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Subagent files live under session directories:
	// <project>/<session>/subagents/agent-<id>.jsonl
	if strings.HasPrefix(sessionID, "agent-") {
		for _, entry := range entries {
			if !isDirOrSymlink(entry, projectsDir) {
				continue
			}
			projDir := filepath.Join(
				projectsDir, entry.Name(),
			)
			sessionDirs, err := os.ReadDir(projDir)
			if err != nil {
				continue
			}
			for _, sd := range sessionDirs {
				if !sd.IsDir() {
					continue
				}
				candidate := filepath.Join(
					projDir, sd.Name(),
					"subagents", target,
				)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}
	}

	return ""
}

// CodeBuddySessionID returns the session ID for a CodeBuddy file.
func CodeBuddySessionID(name string) string {
	return strings.TrimSuffix(name, ".jsonl")
}
