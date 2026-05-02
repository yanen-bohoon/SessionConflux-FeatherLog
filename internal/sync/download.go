package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yanen-bohoon/session-conflux/internal/bundle"
	"github.com/yanen-bohoon/session-conflux/internal/compress"
	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/parser"
)

type RemoteSession struct {
	Key       string
	Computer  string
	Agent     string
	SessionID string
	FileToken string
}

// ListRemoteSessions lists all sessions across all machines on Drive.
func ListRemoteSessions(cfg *config.Config) ([]RemoteSession, error) {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	l1 := cfg.Feishu.FolderToken
	if l1 == "" {
		l1, _ = feishu.FindOrCreateFolder(token, "SessionConflux")
	}

	// List L2: hostname folders
	hosts, err := feishu.ListFiles(token, l1)
	if err != nil {
		return nil, err
	}

	var sessions []RemoteSession
	for _, host := range hosts {
		if host.Type != "folder" {
			continue
		}
		// List L3: baseline + incremental under each host
		l3Files, err := feishu.ListFiles(token, host.Token)
		if err != nil {
			continue
		}
		for _, l3 := range l3Files {
			if l3.Type != "folder" {
				continue
			}
			// List files in baseline / incremental
			files, err := feishu.ListFiles(token, l3.Token)
			if err != nil {
				continue
			}
			for _, f := range files {
				if !strings.HasSuffix(f.Name, ".jsonl.zst") {
					continue
				}
				key := strings.TrimSuffix(f.Name, ".jsonl.zst")
				parts := strings.SplitN(key, "/", 3)
				computer, agent, sessionID := host.Name, "", ""
				if len(parts) >= 1 {
					agent = parts[0]
				}
				if len(parts) >= 2 {
					sessionID = parts[1]
				}
				sessions = append(sessions, RemoteSession{
					Key:       host.Name + "/" + key,
					Computer:  computer,
					Agent:     agent,
					SessionID: sessionID,
					FileToken: f.Token,
				})
			}
		}
	}
	return sessions, nil
}

// DownloadAllSessions downloads from all machines: baseline bundles + incremental files.
func DownloadAllSessions(cfg *config.Config) (int, error) {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return 0, fmt.Errorf("auth: %w", err)
	}

	l1 := cfg.Feishu.FolderToken
	if l1 == "" {
		l1, _ = feishu.FindOrCreateFolder(token, "SessionConflux")
	}
	hosts, err := feishu.ListFiles(token, l1)
	if err != nil {
		return 0, err
	}

	downloaded := 0

	for _, host := range hosts {
		if host.Type != "folder" {
			continue
		}
		fmt.Printf("Machine: %s\n", host.Name)

		// Find baseline and incremental folders
		l3Files, _ := feishu.ListFiles(token, host.Token)
		for _, l3 := range l3Files {
			if l3.Type != "folder" {
				continue
			}
			switch l3.Name {
			case "baseline":
				n := downloadBaseline(token, host.Name, l3.Token)
				downloaded += n
			case "incremental":
				n := downloadIncremental(token, host.Name, l3.Token, cfg)
				downloaded += n
			}
		}
	}
	return downloaded, nil
}

func downloadBaseline(token, hostname, folderToken string) int {
	// Find bundle parts
	allData, err := readBundleParts(token, folderToken)
	if err != nil || len(allData) == 0 {
		return 0
	}

	fmt.Printf("  Extracting baseline (%d KB)...\n", len(allData)/1024)
	files, err := bundle.Unpack(allData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  FAIL unpack: %v\n", err)
		return 0
	}

	n := 0
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
			continue
		}
		n++
	}
	fmt.Printf("  Baseline: %d sessions\n", n)
	return n
}

func readBundleParts(token, folderToken string) ([]byte, error) {
	// Try single bundle first
	files, err := feishu.ListFiles(token, folderToken)
	if err != nil {
		return nil, err
	}

	// Look for bundle.tar.zst or bundle.tar.zst.partNN
	var parts []string
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, bundle.BundleFileName+".") {
			parts = append(parts, f.Name)
		}
	}

	sort.Strings(parts)
	var allData []byte
	for _, name := range parts {
		ft := findFileInList(files, name)
		if ft == "" {
			continue
		}
		data, err := feishu.DownloadFile(token, ft)
		if err != nil {
			continue
		}
		allData = append(allData, data...)
		fmt.Printf("  Part: %s (%d KB)\n", name, len(data)/1024)
	}
	return allData, nil
}

func downloadIncremental(token, hostname, folderToken string, cfg *config.Config) int {
	files, err := feishu.ListFiles(token, folderToken)
	if err != nil {
		return 0
	}

	n := 0
	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".jsonl.zst") {
			continue
		}
		// Parse agent/session_id.jsonl.zst
		key := strings.TrimSuffix(f.Name, ".jsonl.zst")
		parts := strings.SplitN(key, "/", 2)
		agent, sessionID := "", ""
		if len(parts) >= 1 {
			agent = parts[0]
		}
		if len(parts) >= 2 {
			sessionID = parts[1]
		}

		data, err := feishu.DownloadFile(token, f.Token)
		if err != nil {
			continue
		}
		jsonl, err := compress.Decompress(data)
		if err != nil {
			continue
		}
		agentDir := findAgentDir(agent)
		if agentDir == "" {
			continue
		}
		targetDir := filepath.Join(agentDir, "_synced")
		os.MkdirAll(targetDir, 0755)
		os.WriteFile(filepath.Join(targetDir, sessionID+".jsonl"), jsonl, 0644)
		n++
	}
	if n > 0 {
		fmt.Printf("  Incremental: %d sessions\n", n)
	}
	return n
}

func findFileInList(files []feishu.FileInfo, name string) string {
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
		return err
	}
	jsonl, err := compress.Decompress(data)
	if err != nil {
		return err
	}
	agentDir := findAgentDir(session.Agent)
	if agentDir == "" {
		return fmt.Errorf("no dir for %s", session.Agent)
	}
	targetDir := filepath.Join(agentDir, "_synced")
	os.MkdirAll(targetDir, 0755)
	return os.WriteFile(filepath.Join(targetDir, session.SessionID+".jsonl"), jsonl, 0644)
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
