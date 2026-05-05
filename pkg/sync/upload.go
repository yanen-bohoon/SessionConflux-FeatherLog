package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yanen-bohoon/session-conflux/pkg/bundle"
	"github.com/yanen-bohoon/session-conflux/pkg/compress"
	"github.com/yanen-bohoon/session-conflux/pkg/config"
	"github.com/yanen-bohoon/session-conflux/pkg/state"
	"github.com/yanen-bohoon/session-conflux/pkg/transport"
)

const maxChunkSize = 19 * 1024 * 1024

type UploadStats struct {
	Total, Synced, Skipped, Failed int
}

type SyncFile struct {
	Path  string
	Agent string
	Size  int64
	Mtime int64
}

func UploadChanged(t transport.Transport, cfg *config.Config, st *state.Store, files []SyncFile) (*UploadStats, error) {
	hostname, _ := os.Hostname()

	// Ensure host folder exists.
	if err := t.CreateFolder(hostname); err != nil {
		return nil, fmt.Errorf("host folder: %w", err)
	}

	baselinePath := hostname + "/baseline"
	if err := t.CreateFolder(baselinePath); err != nil {
		return nil, fmt.Errorf("baseline folder: %w", err)
	}

	if !baselineHasFiles(t, baselinePath) {
		fmt.Println("First run — uploading baseline bundle...")
		n, err := uploadBaseline(t, hostname, baselinePath, files, cfg.Compression.Level)
		if err != nil {
			return nil, err
		}
		return &UploadStats{Total: n, Synced: n}, nil
	}

	incrPath := hostname + "/incremental"
	if err := t.CreateFolder(incrPath); err != nil {
		return nil, fmt.Errorf("incremental folder: %w", err)
	}

	fmt.Println("Scanning for changes...")
	return uploadIncremental(t, incrPath, hostname, files, st, cfg.Compression.Level)
}

func baselineHasFiles(t transport.Transport, folderPath string) bool {
	files, _ := t.ListFiles(folderPath)
	for _, f := range files {
		if f.Name == bundle.BundleFileName || strings.HasPrefix(f.Name, bundle.BundleFileName+".") {
			return true
		}
	}
	return false
}

func uploadBaseline(t transport.Transport, hostname, baselinePath string, files []SyncFile, level int) (int, error) {
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
		// Derive session ID from filename
		sessionID := strings.TrimSuffix(filepath.Base(f.Path), ".jsonl")
		key := filepath.Join(hostname, f.Agent, sessionID+".jsonl")
		sessionData[key] = data
	}

	fmt.Printf("  compressing %d sessions...\n", len(sessionData))
	archive, err := bundle.Pack(sessionData, level)
	if err != nil {
		return 0, fmt.Errorf("pack: %w", err)
	}

	fmt.Printf("  archive: %d KB\n", len(archive)/1024)

	// Clean old parts.
	oldFiles, _ := t.ListFiles(baselinePath)
	for _, f := range oldFiles {
		t.DeleteFile(baselinePath + "/" + f.Name)
	}

	if len(archive) <= maxChunkSize {
		if err := t.UploadFile(baselinePath, bundle.BundleFileName, archive); err != nil {
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
			if err := t.UploadFile(baselinePath, name, archive[start:end]); err != nil {
				return 0, fmt.Errorf("upload part %d: %w", i+1, err)
			}
			fmt.Printf("  part %d/%d: %d KB\n", i+1, parts, (end-start)/1024)
		}
	}

	return len(files), nil
}

func uploadIncremental(t transport.Transport, incrPath, hostname string, files []SyncFile, st *state.Store, level int) (*UploadStats, error) {
	stats := &UploadStats{Total: len(files)}

	for _, f := range files {
		sessionID := strings.TrimSuffix(filepath.Base(f.Path), ".jsonl")
		key := fmt.Sprintf("%s/%s/%s", hostname, f.Agent, sessionID)

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

		fileName := filepath.Join(f.Agent, sessionID+".jsonl.zst")

		err = t.UploadFile(incrPath, fileName, compressed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: upload %s: %v\n", key, err)
			stats.Failed++
			continue
		}

		st.MarkUploaded(key, f.Size, f.Mtime, "", time.Now().UTC())
		stats.Synced++
		fmt.Printf("  OK: %s (%d KB)\n", key, len(data)/1024)
	}

	st.Save()
	return stats, nil
}
