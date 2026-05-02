package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yanen-bohoon/session-conflux/internal/bundle"
	"github.com/yanen-bohoon/session-conflux/internal/compress"
	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/parser"
)

type RemoteSession struct {
	Key          string
	Computer     string
	Agent        string
	SessionID    string
	FileToken    string
	IsBundlePart bool
}

// ListRemoteSessions lists sessions from Drive (without using slow manifest).
func ListRemoteSessions(cfg *config.Config) ([]RemoteSession, error) {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	files, err := feishu.ListFiles(token, cfg.Feishu.FolderToken)
	if err != nil {
		return nil, err
	}

	var sessions []RemoteSession
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, "bundle.part") {
			sessions = append(sessions, RemoteSession{Key: f.Name, FileToken: f.Token, IsBundlePart: true})
			continue
		}
		if !strings.HasSuffix(f.Name, ".jsonl.zst") {
			continue
		}
		key := strings.TrimSuffix(f.Name, ".jsonl.zst")
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
		sessions = append(sessions, RemoteSession{Key: key, Computer: computer, Agent: agent, SessionID: sessionID, FileToken: f.Token})
	}
	return sessions, nil
}

// DownloadAllSessions downloads bundle parts, assembles, extracts, then downloads individual files.
func DownloadAllSessions(cfg *config.Config) (int, error) {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return 0, fmt.Errorf("auth: %w", err)
	}

	folderToken := cfg.Feishu.FolderToken

	// Step 1: Download bundle. Try single file first, then parts.
	var bundleData []byte
	// Try single bundle
	data, err := feishu.DownloadFile(token, findFileToken(token, folderToken, bundle.BundleFileName))
	if err == nil {
		bundleData = data
		fmt.Printf("  Downloaded bundle (%d KB)\n", len(data)/1024)
	} else {
		// Try parts: bundle.tar.zst.part01, bundle.tar.zst.part02, ...
		for i := 1; ; i++ {
			name := fmt.Sprintf("%s.part%02d", bundle.BundleFileName, i)
			ft := findFileToken(token, folderToken, name)
			if ft == "" {
				break
			}
			data, err := feishu.DownloadFile(token, ft)
			if err != nil {
				break
			}
			bundleData = append(bundleData, data...)
			fmt.Printf("  Downloaded part %d (%d KB)\n", i, len(data)/1024)
		}
	}

	downloaded := 0

	// Extract bundle
	if len(bundleData) > 0 {
		fmt.Printf("  Extracting bundle (%d KB)...\n", len(bundleData)/1024)
		files, err := bundle.Unpack(bundleData)
		if err != nil {
			return 0, fmt.Errorf("unpack: %w", err)
		}
		for name, content := range files {
			agent, sessionID := parseBundleEntry(name)
			if agent == "" {
				continue
			}
			agentDir := findAgentDir(agent)
			if agentDir == "" {
				continue
			}
			if err := bundle.WriteToAgentDir(agent, sessionID, content, agentDir); err != nil {
				fmt.Fprintf(os.Stderr, "  WARN: %s: %v\n", name, err)
				continue
			}
			downloaded++
		}
		fmt.Printf("  Extracted %d sessions from bundle\n", downloaded)
	}

	// Step 2: Download individual files (incremental deltas)
	sessions, _ := ListRemoteSessions(cfg)
	for _, s := range sessions {
		if s.IsBundlePart {
			continue
		}
		if err := DownloadSession(cfg, s); err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: %s: %v\n", s.Key, err)
			continue
		}
		downloaded++
	}
	return downloaded, nil
}

func findFileToken(token, folderToken, name string) string {
	files, _ := feishu.ListFiles(token, folderToken)
	for _, f := range files {
		if f.Name == name {
			return f.Token
		}
	}
	return ""
}

func parseBundleEntry(name string) (agent, sessionID string) {
	name = strings.TrimSuffix(name, ".jsonl")
	parts := strings.SplitN(name, "/", 3)
	if len(parts) >= 2 {
		agent = parts[1]
	}
	if len(parts) >= 3 {
		sessionID = parts[2]
	}
	if idx := strings.LastIndex(sessionID, "/"); idx >= 0 {
		sessionID = sessionID[idx+1:]
	}
	return
}

func DownloadSession(cfg *config.Config, session RemoteSession) error {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	data, err := feishu.DownloadFile(token, session.FileToken)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	jsonl, err := compress.Decompress(data)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	agentDir := findAgentDir(session.Agent)
	if agentDir == "" {
		return fmt.Errorf("no dir for %s", session.Agent)
	}
	targetDir := filepath.Join(agentDir, "_synced")
	os.MkdirAll(targetDir, 0755)
	targetFile := filepath.Join(targetDir, session.SessionID+".jsonl")
	if err := os.WriteFile(targetFile, jsonl, 0644); err != nil {
		return err
	}
	fmt.Printf("  OK: %s\n", session.Key)
	return nil
}

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
