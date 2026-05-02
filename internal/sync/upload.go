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
	Total   int
	Synced  int
	Skipped int
	Failed  int
}

func UploadChanged(cfg *config.Config, st *state.Store) (*UploadStats, error) {
	token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	folderToken := cfg.Feishu.FolderToken
	if folderToken == "" {
		folderToken, err = feishu.FindOrCreateFolder(token, "SessionConflux")
		if err != nil {
			return nil, fmt.Errorf("folder: %w", err)
		}
	}

	hostname, _ := os.Hostname()

	exclude := make(map[string]bool)
	for _, a := range cfg.Agents.Exclude {
		exclude[a] = true
	}

	// Always upload bundle.
	fmt.Println("Uploading bundle...")
	n, err := uploadBundleFast(token, folderToken, hostname, exclude, cfg.Compression.Level)
	if err != nil {
		return nil, err
	}
	return &UploadStats{Total: n, Synced: n}, nil
}

func bundleOnDrive(token, folderToken string) bool {
	files, _ := feishu.ListFiles(token, folderToken)
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, "bundle.part") {
			return true
		}
	}
	return false
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
					if strings.HasPrefix(info.Name(), ".") && path != dir {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(info.Name(), ".jsonl") {
					name := info.Name()
					sid := name[:len(name)-6] // strip .jsonl
					out = append(out, fileEntry{path, def.Type, sid})
				}
				return nil
			})
		}
	}
	return out
}

func uploadBundleFast(token, folderToken, hostname string, exclude map[string]bool, level int) (int, error) {
	files := discoverFiles(exclude)
	fmt.Printf("Packing %d files...\n", len(files))

	// Read files and build tar.zst in one pass
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

	// Upload
	if len(archive) <= maxChunkSize {
		_, err = feishu.UploadFile(token, folderToken, bundle.BundleFileName, archive)
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
			_, err = feishu.UploadFile(token, folderToken, name, archive[start:end])
			if err != nil {
				return 0, fmt.Errorf("upload part %d: %w", i+1, err)
			}
			fmt.Printf("  part %d/%d: %d KB\n", i+1, parts, (end-start)/1024)
		}
	}

	return len(files), nil
}
