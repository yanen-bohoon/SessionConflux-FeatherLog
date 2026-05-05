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

// ProgressFunc receives phase, current/total counts, and optional detail.
// phase: "scanning", "reading", "compressing", "uploading", "listing", "downloading", "extracting", "writing"
// current/total may be 0/0 for indeterminate phases.
type ProgressFunc func(phase string, current, total int, detail string)

// Report calls f if non-nil, making ProgressFunc nil-safe for callers.
func (f ProgressFunc) Report(phase string, current, total int, detail string) {
	if f != nil {
		f(phase, current, total, detail)
	}
}

type UploadStats struct {
	Total   int `json:"total"`
	Synced  int `json:"synced"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

type SyncFile struct {
	Path  string
	Agent string
	Size  int64
	Mtime int64
}

// FileFromDiscovered creates a SyncFile from individual discovered-file fields.
// Fields are passed individually rather than importing agentsview/internal/parser
// so the function is callable from both sides without internal-package restrictions.
func FileFromDiscovered(path, agent string, size, mtime int64) SyncFile {
	return SyncFile{
		Path:  path,
		Agent: agent,
		Size:  size,
		Mtime: mtime,
	}
}

func UploadChanged(t transport.Transport, cfg *config.Config, st *state.Store, files []SyncFile, onProgress ProgressFunc) (*UploadStats, error) {
	hostname, _ := os.Hostname()

	// Ensure host folder exists.
	if err := t.CreateFolder(hostname); err != nil {
		return nil, fmt.Errorf("host folder: %w", err)
	}

	baselinePath := hostname + "/baseline"
	if err := t.CreateFolder(baselinePath); err != nil {
		return nil, fmt.Errorf("baseline folder: %w", err)
	}

	if len(st.All()) == 0 {
		fmt.Println("First upload — uploading baseline bundle...")
		n, err := uploadBaseline(t, hostname, baselinePath, files, cfg.Compression.Level, onProgress)
		if err != nil {
			return nil, err
		}
		return &UploadStats{Total: n, Synced: n}, nil
	}

	incrPath := hostname + "/incremental"
	if err := t.CreateFolder(incrPath); err != nil {
		return nil, fmt.Errorf("incremental folder: %w", err)
	}

	onProgress.Report("scanning", 0, 0, "")
	fmt.Println("Scanning for changes...")
	return uploadIncremental(t, incrPath, hostname, files, st, cfg.Compression.Level, onProgress)
}

func uploadBaseline(t transport.Transport, hostname, baselinePath string, files []SyncFile, level int, onProgress ProgressFunc) (int, error) {
	fmt.Printf("Packing %d files...\n", len(files))

	sessionData := make(map[string][]byte)
	onProgress.Report("reading", 0, len(files), "")
	for i, f := range files {
		if i > 0 && i%50 == 0 {
			fmt.Printf("  reading %d/%d...\n", i, len(files))
			onProgress.Report("reading", i, len(files), "")
		}
		data, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		sessionID := strings.TrimSuffix(filepath.Base(f.Path), ".jsonl")
		key := filepath.Join(hostname, f.Agent, sessionID+".jsonl")
		sessionData[key] = data
	}
	onProgress.Report("reading", len(files), len(files), "")

	fmt.Printf("  compressing %d sessions...\n", len(sessionData))
	onProgress.Report("compressing", 0, 0, fmt.Sprintf("%d sessions", len(sessionData)))
	archive, err := bundle.Pack(sessionData, level)
	if err != nil {
		return 0, fmt.Errorf("pack: %w", err)
	}

	archiveKB := len(archive) / 1024
	fmt.Printf("  archive: %d KB\n", archiveKB)
	onProgress.Report("compressing", 0, 0, fmt.Sprintf("%d KB", archiveKB))

	// Clean old parts.
	oldFiles, _ := t.ListFiles(baselinePath)
	for _, f := range oldFiles {
		t.DeleteFile(baselinePath + "/" + f.Name)
	}

	chunkSize := int(t.MaxChunkSize())
	if chunkSize <= 0 || len(archive) <= chunkSize {
		onProgress.Report("uploading", 0, 0, "baseline bundle")
		if err := t.UploadFile(baselinePath, bundle.BundleFileName, archive); err != nil {
			return 0, fmt.Errorf("upload: %w", err)
		}
		fmt.Printf("  uploaded bundle.tar.zst\n")
	} else {
		parts := (len(archive) + chunkSize - 1) / chunkSize
		fmt.Printf("  splitting into %d parts...\n", parts)
		for i := 0; i < parts; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if end > len(archive) {
				end = len(archive)
			}
			name := fmt.Sprintf("%s.part%02d", bundle.BundleFileName, i+1)
			onProgress.Report("uploading", i+1, parts, fmt.Sprintf("part %d/%d", i+1, parts))
			if err := t.UploadFile(baselinePath, name, archive[start:end]); err != nil {
				return 0, fmt.Errorf("upload part %d: %w", i+1, err)
			}
			fmt.Printf("  part %d/%d: %d KB\n", i+1, parts, (end-start)/1024)
		}
	}

	return len(files), nil
}

func uploadIncremental(t transport.Transport, incrPath, hostname string, files []SyncFile, st *state.Store, level int, onProgress ProgressFunc) (*UploadStats, error) {
	stats := &UploadStats{Total: len(files)}
	processed := 0

	for _, f := range files {
		sessionID := strings.TrimSuffix(filepath.Base(f.Path), ".jsonl")
		key := fmt.Sprintf("%s/%s/%s", hostname, f.Agent, sessionID)

		if !st.HasChanged(key, f.Size, f.Mtime) {
			stats.Skipped++
			processed++
			onProgress.Report("uploading", processed, len(files), key+" (skipped)")
			continue
		}

		data, err := os.ReadFile(f.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: skip %s: %v\n", key, err)
			stats.Failed++
			processed++
			onProgress.Report("uploading", processed, len(files), key+" (failed)")
			continue
		}

		compressed, err := compress.Compress(data, level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: compress %s: %v\n", key, err)
			stats.Failed++
			processed++
			onProgress.Report("uploading", processed, len(files), key+" (failed)")
			continue
		}

		fileName := filepath.Join(f.Agent, sessionID+".jsonl.zst")

		err = t.UploadFile(incrPath, fileName, compressed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL: upload %s: %v\n", key, err)
			stats.Failed++
			processed++
			onProgress.Report("uploading", processed, len(files), key+" (failed)")
			continue
		}

		st.MarkUploaded(key, f.Size, f.Mtime, "", time.Now().UTC())
		stats.Synced++
		processed++
		onProgress.Report("uploading", processed, len(files), f.Agent+"/"+sessionID)
		fmt.Printf("  OK: %s (%d KB)\n", key, len(data)/1024)
	}

	st.Save()
	return stats, nil
}
