package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yanen-bohoon/session-conflux/internal/bundle"
	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/parser"
	"github.com/yanen-bohoon/session-conflux/internal/state"
)

const maxChunkSize = 19 * 1024 * 1024

type UploadStats struct {
	Total, Synced, Skipped, Failed int
}

func UploadChanged(cfg *config.Config, st *state.Store) (*UploadStats, error) {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	l1 := cfg.Feishu.FolderToken
	if l1 == "" {
		l1, err = feishu.FindOrCreateFolder(token, "SessionConflux")
		if err != nil {
			return nil, fmt.Errorf("L1 folder: %w", err)
		}
	}

	hostname, _ := os.Hostname()

	exclude := make(map[string]bool)
	for _, a := range cfg.Agents.Exclude {
		exclude[a] = true
	}

	l2, err := feishu.FindOrCreateFolder(token, hostname, l1)
	if err != nil {
		return nil, fmt.Errorf("L2 folder: %w", err)
	}
	baseline, err := feishu.FindOrCreateFolder(token, "baseline", l2)
	if err != nil {
		return nil, fmt.Errorf("baseline folder: %w", err)
	}

	fmt.Println("Uploading bundle...")
	n, err := uploadBundle(token, baseline, hostname, exclude, cfg.Compression.Level)
	if err != nil {
		return nil, err
	}
	return &UploadStats{Total: n, Synced: n}, nil
}

type fileEntry struct{ Path, Agent, SessionID string }

func discoverFiles(exclude map[string]bool) []fileEntry {
	var out []fileEntry
	for _, def := range parser.AllAgents {
		if exclude[def.Type] {
			continue
		}
		for _, dir := range parser.ResolveAgentDirs(def) {
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					if info.Name() == "_synced" || (strings.HasPrefix(info.Name(), ".") && path != dir) {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(info.Name(), ".jsonl") {
					name := info.Name()
					sid := name[:len(name)-6]
					out = append(out, fileEntry{path, def.Type, sid})
				}
				return nil
			})
		}
	}
	return out
}

func uploadBundle(token, baselineToken, hostname string, exclude map[string]bool, level int) (int, error) {
	files := discoverFiles(exclude)
	fmt.Printf("Packing %d files...\n", len(files))

	sessionData := make(map[string][]byte)
	for i, f := range files {
		if i > 0 && i%50 == 0 {
			fmt.Printf("  reading %d/%d...\n", i, len(files))
		}
		data, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		key := filepath.Join(hostname, f.Agent, f.SessionID+".jsonl")
		sessionData[key] = data
	}

	fmt.Printf("  compressing %d sessions...\n", len(sessionData))
	archive, err := bundle.Pack(sessionData, level)
	if err != nil {
		return 0, fmt.Errorf("pack: %w", err)
	}

	fmt.Printf("  archive: %d KB\n", len(archive)/1024)

	// Clean old bundle parts before uploading new ones
	oldFiles, _ := feishu.ListFiles(token, baselineToken)
	for _, f := range oldFiles {
		feishu.DeleteFile(token, f.Token)
	}

	if len(archive) <= maxChunkSize {
		_, err = feishu.UploadFile(token, baselineToken, bundle.BundleFileName, archive)
		if err != nil {
			return 0, fmt.Errorf("upload: %w", err)
		}
		fmt.Printf("  uploaded bundle.tar.zst\n")
	} else {
		parts := (len(archive) + maxChunkSize - 1) / maxChunkSize
		fmt.Printf("  splitting into %d parts...\n", parts)
		for i := 0; i < parts; i++ {
			start := i * maxChunkSize
			end := start + maxChunkSize
			if end > len(archive) {
				end = len(archive)
			}
			name := fmt.Sprintf("%s.part%02d", bundle.BundleFileName, i+1)
			_, err = feishu.UploadFile(token, baselineToken, name, archive[start:end])
			if err != nil {
				return 0, fmt.Errorf("upload part %d: %w", i+1, err)
			}
			fmt.Printf("  part %d/%d: %d KB\n", i+1, parts, (end-start)/1024)
		}
	}

	return len(files), nil
}
