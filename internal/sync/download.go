package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yanen-bohoon/session-conflux/internal/compress"
	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/manifest"
	"github.com/yanen-bohoon/session-conflux/internal/parser"
)

// RemoteSession represents a session available for download from Feishu Drive.
type RemoteSession struct {
	Key          string // "computer/agent/session_id"
	Computer     string
	Agent        string
	SessionID    string
	Title        string
	MessageCount int
	FileToken    string
}

// ListRemoteSessions fetches and returns all remote sessions from the manifest.
func ListRemoteSessions(cfg *config.Config) ([]RemoteSession, error) {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	folderToken := cfg.Feishu.FolderToken

	m, err := manifest.Download(token, folderToken)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if m == nil {
		return nil, nil
	}

	var sessions []RemoteSession
	for key, entry := range m.Sessions {
		parts := strings.SplitN(key, "/", 3)
		computer, agent, sessionID := "", "", ""
		if len(parts) >= 1 {
			computer = parts[0]
		}
		if len(parts) >= 2 {
			agent = parts[1]
		}
		if len(parts) >= 3 {
			sessionID = parts[2]
		}
		sessions = append(sessions, RemoteSession{
			Key:          key,
			Computer:     computer,
			Agent:        agent,
			SessionID:    sessionID,
			Title:        entry.Title,
			MessageCount: entry.MessageCount,
			FileToken:    entry.FileToken,
		})
	}
	return sessions, nil
}

// DownloadSession downloads a single session from Feishu Drive and writes it
// to the appropriate agent directory for AgentsView to discover.
func DownloadSession(cfg *config.Config, session RemoteSession) error {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	// Download compressed file
	data, err := feishu.DownloadFile(token, session.FileToken)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}

	// Decompress
	jsonl, err := compress.Decompress(data)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}

	// Find the agent's local directory
	agentDir := findAgentDir(session.Agent)
	if agentDir == "" {
		return fmt.Errorf("no local directory found for agent %s", session.Agent)
	}

	// Write JSONL to agent directory
	// For Claude/Codex: write to a projects/sessions subdirectory
	// The filename should be the session_id.jsonl
	targetDir := resolveSessionDir(agentDir, session.Agent)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	targetFile := filepath.Join(targetDir, session.SessionID+".jsonl")
	if err := os.WriteFile(targetFile, jsonl, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("  Downloaded: %s -> %s\n", session.Key, targetFile)
	return nil
}

// DownloadAllSessions downloads all remote sessions.
func DownloadAllSessions(cfg *config.Config) (int, error) {
	sessions, err := ListRemoteSessions(cfg)
	if err != nil {
		return 0, err
	}
	if len(sessions) == 0 {
		fmt.Println("No remote sessions found.")
		return 0, nil
	}

	downloaded := 0
	for _, s := range sessions {
		if err := DownloadSession(cfg, s); err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: %s: %v\n", s.Key, err)
			continue
		}
		downloaded++
	}
	return downloaded, nil
}

// findAgentDir returns the first existing base directory for an agent.
func findAgentDir(agent string) string {
	for _, def := range parser.AllAgents {
		if def.Type == agent {
			dirs := parser.ResolveAgentDirs(def)
			if len(dirs) > 0 {
				return dirs[0]
			}
			break
		}
	}
	return ""
}

// resolveSessionDir determines where to write the session file.
// For directory-style agents (claude, cursor, etc.), sessions live
// in project subdirectories. We write to a "_remote" subdirectory
// to avoid clashing with local projects while still being discoverable.
func resolveSessionDir(agentDir, agent string) string {
	return filepath.Join(agentDir, "_synced")
}
