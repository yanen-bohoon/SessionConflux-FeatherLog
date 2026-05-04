package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yanen-bohoon/session-conflux/internal/bundle"
	"github.com/yanen-bohoon/session-conflux/internal/compress"
	"github.com/yanen-bohoon/session-conflux/internal/registry"
	"github.com/yanen-bohoon/session-conflux/internal/state"
	"github.com/yanen-bohoon/session-conflux/internal/transport"
)

type RemoteSession struct {
	Key       string
	Computer  string
	Agent     string
	SessionID string
	Path      string // transport path, e.g. "mac-studio/incremental/claude/sess-abc.jsonl.zst"
}

// ListRemoteSessions lists all sessions across all machines on the transport.
func ListRemoteSessions(t transport.Transport) ([]RemoteSession, error) {
	hosts, err := t.ListFiles("")
	if err != nil {
		return nil, err
	}

	var sessions []RemoteSession
	for _, host := range hosts {
		if !host.IsDir {
			continue
		}
		l3Files, err := t.ListFiles(host.Name)
		if err != nil {
			continue
		}
		for _, l3 := range l3Files {
			if !l3.IsDir {
				continue
			}
			files, err := t.ListFiles(host.Name + "/" + l3.Name)
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
					Path:      host.Name + "/" + l3.Name + "/" + f.Name,
				})
			}
		}
	}
	return sessions, nil
}

// DownloadAllSessions downloads from all machines: baseline bundles + incremental files.
// Skips the local machine to avoid downloading its own sessions back.
func DownloadAllSessions(t transport.Transport) (int, error) {
	st, err := state.Load()
	if err != nil {
		return 0, fmt.Errorf("load state: %w", err)
	}

	localHost, _ := os.Hostname()

	hosts, err := t.ListFiles("")
	if err != nil {
		return 0, err
	}

	downloaded := 0
	now := time.Now().UTC()

	for _, host := range hosts {
		if !host.IsDir {
			continue
		}

		if host.Name == localHost {
			fmt.Printf("Machine: %s (skipped — local)\n", host.Name)
			continue
		}

		fmt.Printf("Machine: %s\n", host.Name)

		l3Files, err := t.ListFiles(host.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: list %s contents: %v\n", host.Name, err)
			continue
		}
		for _, l3 := range l3Files {
			if !l3.IsDir {
				continue
			}
			switch l3.Name {
			case "baseline":
				n := downloadBaseline(t, host.Name, host.Name+"/baseline", st, now)
				downloaded += n
			case "incremental":
				n := downloadIncremental(t, host.Name, host.Name+"/incremental", st, now)
				downloaded += n
			}
		}
	}

	st.Save()
	return downloaded, nil
}

func downloadBaseline(t transport.Transport, hostname, baselinePath string, st *state.Store, now time.Time) int {
	files, err := t.ListFiles(baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  WARN: list baseline: %v\n", err)
		return 0
	}

	var partNames []string
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, bundle.BundleFileName+".") {
			partNames = append(partNames, f.Name)
		}
	}
	if len(partNames) == 0 {
		return 0
	}

	// Build a composite check key from sorted parts for change detection.
	compositeToken := strings.Join(partNames, ",")
	baselineKey := hostname + "/baseline"

	if !st.NeedsDownload(baselineKey, compositeToken) {
		fmt.Println("  Baseline: up to date")
		return 0
	}

	// Sort for deterministic order.
	sorted := make([]string, len(partNames))
	copy(sorted, partNames)
	// Simple bubble sort since part count is tiny.
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var allData []byte
	for i, name := range sorted {
		label := fmt.Sprintf("Baseline: [%d/%d] %s", i+1, len(sorted), name)
		fmt.Printf("\r  %s", label)
		data, err := t.DownloadFile(baselinePath + "/" + name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  WARN: download %s: %v\n", name, err)
			continue
		}
		fmt.Printf("\r  %s  %s", label, formatBytes(int64(len(data))))
		allData = append(allData, data...)
	}
	fmt.Print("\n")

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
		if err := writeToAgentDir(hostname, agent, sessionID, content, agentDir); err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: write %s/%s/%s: %v\n", hostname, agent, sessionID, err)
			continue
		}
		n++
	}
	st.MarkDownloaded(baselineKey, compositeToken, now)
	fmt.Printf("  Baseline: %d sessions\n", n)
	return n
}

func downloadIncremental(t transport.Transport, hostname, incrPath string, st *state.Store, now time.Time) int {
	files, err := t.ListFiles(incrPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  WARN: list incremental: %v\n", err)
		return 0
	}

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

		if !st.NeedsDownload(dlKey, f.Name) {
			skipped++
			continue
		}

		label := fmt.Sprintf("%s/%s.jsonl.zst", agent, sessionID)
		fmt.Printf("\r  Incremental: [%d/%d] %s",
			sessionCount+skipped+1, total, label)
		data, err := t.DownloadFile(incrPath + "/" + f.Name)
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
		if err := writeToAgentDir(hostname, agent, sessionID, jsonl, agentDir); err != nil {
			fmt.Fprintf(os.Stderr, "\n  WARN: write %s: %v\n", dlKey, err)
			continue
		}
		st.MarkDownloaded(dlKey, f.Name, now)
		sessionCount++
	}
	if sessionCount+skipped > 0 {
		fmt.Printf("\r  Incremental: %d sessions (%d new, %d skipped)\n",
			sessionCount+skipped, sessionCount, skipped)
	}
	return sessionCount
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

func DownloadSession(t transport.Transport, session RemoteSession) error {
	fmt.Printf("  Downloading %s...", session.Key)
	data, err := t.DownloadFile(session.Path)
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
	return writeToAgentDir(session.Computer, session.Agent, session.SessionID, jsonl, agentDir)
}

func writeToAgentDir(hostname, agent, sessionID string, data []byte, agentDir string) error {
	targetDir := filepath.Join(agentDir, "_synced", hostname)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	targetFile := filepath.Join(targetDir, sessionID+".jsonl")
	if err := os.WriteFile(targetFile, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
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
