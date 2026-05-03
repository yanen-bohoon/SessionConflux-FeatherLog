package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yanen-bohoon/session-conflux/internal/bundle"
	"github.com/yanen-bohoon/session-conflux/internal/compress"
	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/registry"
	"github.com/yanen-bohoon/session-conflux/internal/state"
)

const maxChunkSize = 19 * 1024 * 1024

type UploadStats struct {
	Total, Synced, Skipped, Failed int
}

func UploadChanged(client *feishu.Client, cfg *config.Config, st *state.Store) (*UploadStats, error) {
	l1 := cfg.Feishu.FolderToken
	if l1 == "" {
		var err error
		l1, err = client.FindOrCreateFolder("SessionConflux")
		if err != nil {
			return nil, fmt.Errorf("L1 folder: %w", err)
		}
	}

	hostname, _ := os.Hostname()

	exclude := make(map[string]bool)
	for _, a := range cfg.Agents.Exclude {
		exclude[a] = true
	}

	l2, err := client.FindOrCreateFolder(hostname, l1)
	if err != nil {
		return nil, fmt.Errorf("L2 folder: %w", err)
	}

	baseline, err := client.FindOrCreateFolder("baseline", l2)
	if err != nil {
		return nil, fmt.Errorf("baseline folder: %w", err)
	}

	// Check if baseline already has bundle files
	hasBaseline := baselineHasFiles(client, baseline)

	if !hasBaseline {
		fmt.Println("First run — uploading baseline bundle...")
		n, err := uploadBaseline(client, baseline, hostname, exclude, cfg.Compression.Level)
		if err != nil {
			return nil, err
		}
		return &UploadStats{Total: n, Synced: n}, nil
	}

	// Incremental
	incr, err := client.FindOrCreateFolder("incremental", l2)
	if err != nil {
		return nil, fmt.Errorf("incremental folder: %w", err)
	}

	fmt.Println("Scanning for changes...")
	return uploadIncremental(client, incr, hostname, exclude, st, cfg.Compression.Level)
}

func baselineHasFiles(client *feishu.Client, folderToken string) bool {
	files, _ := client.ListFiles(folderToken)
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, bundle.BundleFileName+".") {
			return true
		}
	}
	return false
}

type fileEntry struct {
	Path, Agent, SessionID string
	Size                   int64
	Mtime                  int64
}

func discoverFiles(exclude map[string]bool) []fileEntry {
	var out []fileEntry
	for _, def := range registry.AllAgents {
		if exclude[def.Type] {
			continue
		}
		for _, dir := range registry.ResolveAgentDirs(def) {
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
					out = append(out, fileEntry{
						Path:      path,
						Agent:     def.Type,
						SessionID: sid,
						Size:      info.Size(),
						Mtime:     info.ModTime().UnixNano(),
					})
				}
				return nil
			})
		}
	}
	return out
}

func uploadBaseline(client *feishu.Client, baselineToken, hostname string, exclude map[string]bool, level int) (int, error) {
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

	// Clean old parts
	oldFiles, _ := client.ListFiles(baselineToken)
	for _, f := range oldFiles {
		client.DeleteFile(f.Token)
	}

	if len(archive) <= maxChunkSize {
		_, err = client.UploadFile(baselineToken, bundle.BundleFileName, archive)
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
			_, err = client.UploadFile(baselineToken, name, archive[start:end])
			if err != nil {
				return 0, fmt.Errorf("upload part %d: %w", i+1, err)
			}
			fmt.Printf("  part %d/%d: %d KB\n", i+1, parts, (end-start)/1024)
		}
	}

	return len(files), nil
}

func uploadIncremental(client *feishu.Client, incrToken, hostname string, exclude map[string]bool, st *state.Store, level int) (*UploadStats, error) {
	files := discoverFiles(exclude)
	stats := &UploadStats{Total: len(files)}

	for _, f := range files {
		key := fmt.Sprintf("%s/%s/%s", hostname, f.Agent, f.SessionID)

		if !st.HasChanged(key, f.Size, f.Mtime) {
			stats.Skipped++
			continue
		}

		data, err := os.ReadFile(f.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: skip %s: %v\n", key, err)
			stats.Failed++
			continue
		}

		compressed, err := compress.Compress(data, level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: compress %s: %v\n", key, err)
			stats.Failed++
			continue
		}

		// Incremental files: agent/session_id.jsonl.zst
		fileName := filepath.Join(f.Agent, f.SessionID+".jsonl.zst")

		for attempt := 1; attempt <= 3; attempt++ {
			_, err = client.UploadFile(incrToken, fileName, compressed)
			if err == nil {
				break
			}
			if attempt < 3 {
				time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: upload %s: %v\n", key, err)
			stats.Failed++
			continue
		}

		st.MarkUploaded(key, f.Size, f.Mtime, "", time.Now().UTC().Format(time.RFC3339))
		stats.Synced++
		fmt.Printf("  OK: %s (%d KB)\n", key, len(data)/1024)
	}

	st.Save()
	return stats, nil
}
