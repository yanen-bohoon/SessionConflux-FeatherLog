package sync

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/yanen-bohoon/session-conflux/internal/bundle"
	"github.com/yanen-bohoon/session-conflux/internal/compress"
	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/registry"
	"github.com/yanen-bohoon/session-conflux/internal/state"
)

type RemoteSession struct {
	Key       string
	Computer  string
	Agent     string
	SessionID string
	FileToken string
}

// ListRemoteSessions lists all sessions across all machines on Drive.
func ListRemoteSessions(client *feishu.Client, cfg *config.Config) ([]RemoteSession, error) {
	l1 := cfg.Feishu.FolderToken
	if l1 == "" {
		var err error
		l1, err = client.FindOrCreateFolder("SessionConflux")
		if err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
	}

	// List L2: hostname folders
	hosts, err := client.ListFiles(l1)
	if err != nil {
		return nil, err
	}

	var sessions []RemoteSession
	for _, host := range hosts {
		if host.Type != "folder" {
			continue
		}
		// List L3: baseline + incremental under each host
		l3Files, err := client.ListFiles(host.Token)
		if err != nil {
			continue
		}
		for _, l3 := range l3Files {
			if l3.Type != "folder" {
				continue
			}
			// List files in baseline / incremental
			files, err := client.ListFiles(l3.Token)
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
// Skips the local machine to avoid downloading its own sessions back.
func DownloadAllSessions(client *feishu.Client, cfg *config.Config) (int, error) {
	st, err := state.Load()
	if err != nil {
		return 0, fmt.Errorf("load state: %w", err)
	}

	localHost, _ := os.Hostname()

	l1 := cfg.Feishu.FolderToken
	if l1 == "" {
		var err error
		l1, err = client.FindOrCreateFolder("SessionConflux")
		if err != nil {
			return 0, err
		}
	}
	hosts, err := client.ListFiles(l1)
	if err != nil {
		return 0, err
	}

	downloaded := 0
	now := time.Now().UTC().Format(time.RFC3339)

	for _, host := range hosts {
		if host.Type != "folder" {
			continue
		}

		// Skip sessions from this machine — no need to download them back.
		if host.Name == localHost {
			fmt.Printf("Machine: %s (skipped — local)\n", host.Name)
			continue
		}

		fmt.Printf("Machine: %s\n", host.Name)

		// Find baseline and incremental folders
		l3Files, err := client.ListFiles(host.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: list %s contents: %v\n", host.Name, err)
			continue
		}
		for _, l3 := range l3Files {
			if l3.Type != "folder" {
				continue
			}
			switch l3.Name {
			case "baseline":
				n := downloadBaseline(client, host.Name, l3.Token, st, now)
				downloaded += n
			case "incremental":
				n := downloadIncremental(client, host.Name, l3.Token, st, now)
				downloaded += n
			}
		}
	}

	st.Save()
	return downloaded, nil
}

func downloadBaseline(client *feishu.Client, hostname, folderToken string, st *state.Store, now string) int {
	files, err := client.ListFiles(folderToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  WARN: list baseline: %v\n", err)
		return 0
	}

	// Build a composite token from all parts for change detection.
	var partTokens []string
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, bundle.BundleFileName+".") {
			partTokens = append(partTokens, f.Token)
		}
	}
	if len(partTokens) == 0 {
		return 0
	}
	compositeToken := strings.Join(partTokens, ",")
	baselineKey := hostname + "/baseline"

	if !st.NeedsDownload(baselineKey, compositeToken) {
		fmt.Println("  Baseline: up to date")
		return 0
	}

	allData, err := readBundlePartsData(client, folderToken, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  WARN: read bundle parts: %v\n", err)
		return 0
	}
	if len(allData) == 0 {
		return 0
	}

	fmt.Printf("  Extracting baseline (%d KB)...\n", len(allData)/1024)
	entries, err := bundle.Unpack(allData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  FAIL unpack: %v\n", err)
		return 0
	}

	n := 0
	for name, content := range entries {
		_, agent, sessionID := parseBundleEntry(name)
		if agent == "" {
			continue
		}
		agentDir := findAgentDir(agent)
		if agentDir == "" {
			continue
		}
		if err := bundle.WriteToAgentDir(hostname, agent, sessionID, content, agentDir); err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: write %s/%s/%s: %v\n", hostname, agent, sessionID, err)
			continue
		}
		n++
	}
	st.MarkDownloaded(baselineKey, compositeToken, now)
	fmt.Printf("  Baseline: %d sessions\n", n)
	return n
}

func readBundlePartsData(client *feishu.Client, folderToken string, files []feishu.FileInfo) ([]byte, error) {
	// Look for bundle.tar.zst or bundle.tar.zst.partNN
	var parts []string
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, bundle.BundleFileName+".") {
			parts = append(parts, f.Name)
		}
	}

	sort.Strings(parts)
	var allData []byte
	for i, name := range parts {
		ft := findFileInList(files, name)
		if ft == "" {
			continue
		}

		fmt.Printf("\r  Baseline: [%d/%d] %s", i+1, len(parts), name)
		data, err := client.DownloadFile(ft, func(downloaded, total int64) {
			fmt.Printf("\r  Baseline: [%d/%d] %s  %s / %s",
				i+1, len(parts), name,
				formatBytes(downloaded), formatBytesOrUnknown(total))
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  WARN: download %s: %v\n", name, err)
			continue
		}
		allData = append(allData, data...)
	}
	fmt.Print("\n")
	return allData, nil
}

func downloadIncremental(client *feishu.Client, hostname, folderToken string, st *state.Store, now string) int {
	files, err := client.ListFiles(folderToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  WARN: list incremental: %v\n", err)
		return 0
	}

	// Count .jsonl.zst files for progress total.
	total := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name, ".jsonl.zst") {
			total++
		}
	}

	sessionCount := 0
	skipped := 0
	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".jsonl.zst") {
			continue
		}
		// Parse agent/session_id.jsonl.zst
		nameKey := strings.TrimSuffix(f.Name, ".jsonl.zst")
		parts := strings.SplitN(nameKey, "/", 2)
		agent, sessionID := "", ""
		if len(parts) >= 1 {
			agent = parts[0]
		}
		if len(parts) >= 2 {
			sessionID = parts[1]
		}

		dlKey := hostname + "/" + agent + "/" + sessionID

		if !st.NeedsDownload(dlKey, f.Token) {
			skipped++
			continue
		}

		label := fmt.Sprintf("%s/%s.jsonl.zst", agent, sessionID)
		fmt.Printf("\r  Incremental: [%d/%d] %s",
			sessionCount+skipped+1, total, label)
		data, err := client.DownloadFile(f.Token, func(downloaded, totalBytes int64) {
			fmt.Printf("\r  Incremental: [%d/%d] %s  %s / %s",
				sessionCount+skipped+1, total, label,
				formatBytes(downloaded), formatBytesOrUnknown(totalBytes))
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  WARN: download %s: %v\n", dlKey, err)
			continue
		}
		jsonl, err := compress.Decompress(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  WARN: decompress %s: %v\n", dlKey, err)
			continue
		}
		agentDir := findAgentDir(agent)
		if agentDir == "" {
			continue
		}
		if err := bundle.WriteToAgentDir(hostname, agent, sessionID, jsonl, agentDir); err != nil {
			fmt.Fprintf(os.Stderr, "\n  WARN: write %s: %v\n", dlKey, err)
			continue
		}
		st.MarkDownloaded(dlKey, f.Token, now)
		sessionCount++
	}
	if sessionCount+skipped > 0 {
		fmt.Printf("\r  Incremental: %d sessions (%d new, %d skipped)\n",
			sessionCount+skipped, sessionCount, skipped)
	}
	return sessionCount
}

func findFileInList(files []feishu.FileInfo, name string) string {
	for _, f := range files {
		if f.Name == name {
			return f.Token
		}
	}
	return ""
}

func parseBundleEntry(name string) (hostname, agent, sessionID string) {
	name = strings.TrimSuffix(name, ".jsonl")
	parts := strings.SplitN(name, "/", 3)
	if len(parts) >= 1 {
		hostname = parts[0]
	}
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

func DownloadSession(client *feishu.Client, session RemoteSession) error {
	fmt.Printf("  Downloading %s...", session.Key)
	data, err := client.DownloadFile(session.FileToken, func(downloaded, total int64) {
		fmt.Printf("\r  Downloading %s  %s / %s",
			session.Key,
			formatBytes(downloaded), formatBytesOrUnknown(total))
	})
	if err != nil {
		fmt.Print("\n")
		return err
	}
	fmt.Printf("\r  Downloading %s  %s done\n", session.Key, formatBytes(int64(len(data))))
	jsonl, err := compress.Decompress(data)
	if err != nil {
		return err
	}
	agentDir := findAgentDir(session.Agent)
	if agentDir == "" {
		return fmt.Errorf("no dir for %s", session.Agent)
	}
	return bundle.WriteToAgentDir(session.Computer, session.Agent, session.SessionID, jsonl, agentDir)
}

func findAgentDir(agent string) string {
	for _, def := range registry.AllAgents {
		if def.Type == agent {
			dirs := registry.ResolveAgentDirs(def)
			if len(dirs) > 0 {
				return dirs[0]
			}
			break
		}
	}
	return ""
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 2 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KM"[exp])
}

func formatBytesOrUnknown(n int64) string {
	if n <= 0 {
		return "?? B"
	}
	return formatBytes(n)
}
