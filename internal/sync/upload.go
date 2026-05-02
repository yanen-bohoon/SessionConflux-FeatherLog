package sync

import (
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/yanen-bohoon/session-conflux/internal/compress"
    "github.com/yanen-bohoon/session-conflux/internal/config"
    "github.com/yanen-bohoon/session-conflux/internal/feishu"
    "github.com/yanen-bohoon/session-conflux/internal/manifest"
    "github.com/yanen-bohoon/session-conflux/internal/scanner"
    "github.com/yanen-bohoon/session-conflux/internal/state"
)

// UploadStats holds upload run statistics.
type UploadStats struct {
    Total   int
    Synced  int
    Skipped int
    Failed  int
}

// UploadChanged scans local sessions and uploads any with new messages.
func UploadChanged(cfg *config.Config, st *state.Store) (*UploadStats, error) {
    token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
    if err != nil {
        return nil, fmt.Errorf("auth: %w", err)
    }

    // Ensure root folder exists
    folderToken := cfg.Feishu.FolderToken
    if folderToken == "" {
        folderToken, err = feishu.FindOrCreateFolder(token, "SessionConflux")
        if err != nil {
            return nil, fmt.Errorf("folder: %w", err)
        }
    }

    // Download current manifest to get existing manifest file token
    var manifestFileToken string
    m, err := manifest.Download(token, folderToken)
    if err != nil {
        return nil, fmt.Errorf("manifest download: %w", err)
    }
    if m == nil {
        m = manifest.New()
    }
    // Find existing manifest file token by listing folder
    files, _ := feishu.ListFiles(token, folderToken)
    for _, f := range files {
        if f.Name == manifest.ManifestFileName {
            manifestFileToken = f.Token
            break
        }
    }

    // Get hostname for session key
    hostname, _ := os.Hostname()

    // Build exclude set
    exclude := make(map[string]bool)
    for _, a := range cfg.Agents.Exclude {
        exclude[a] = true
    }

    // Scan sessions
    sessions, err := scanner.Scan(exclude)
    if err != nil {
        return nil, fmt.Errorf("scan: %w", err)
    }

    stats := &UploadStats{Total: len(sessions)}

    for _, sess := range sessions {
        key := fmt.Sprintf("%s/%s/%s", hostname, sess.Agent, sess.SessionID)

        // Check if changed
        if !st.HasChanged(key, sess.MessageCount) {
            stats.Skipped++
            continue
        }

        // Read file
        data, err := os.ReadFile(sess.Path)
        if err != nil {
            fmt.Fprintf(os.Stderr, "  WARN: skip %s: %v\n", key, err)
            stats.Failed++
            continue
        }

        // Compress
        compressed, err := compress.Compress(data, cfg.Compression.Level)
        if err != nil {
            fmt.Fprintf(os.Stderr, "  WARN: compress %s: %v\n", key, err)
            stats.Failed++
            continue
        }

        // Build file path: computer/agent/session_id.jsonl.zst
        fileName := filepath.Join(hostname, sess.Agent, sess.SessionID+".jsonl.zst")

        // Upload with retries
        var fileToken string
        for attempt := 1; attempt <= 3; attempt++ {
            fileToken, err = feishu.UploadFile(token, folderToken, fileName, compressed)
            if err == nil {
                break
            }
            if attempt < 3 {
                time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
                // Refresh token on retry
                token, _ = feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
            }
        }
        if err != nil {
            fmt.Fprintf(os.Stderr, "  FAIL: upload %s: %v\n", key, err)
            stats.Failed++
            continue
        }

        // Update manifest
        manifestFileToken, err = manifest.UpsertSession(token, folderToken, manifestFileToken, key, manifest.SessionEntry{
            Title:        sess.Title,
            MessageCount: sess.MessageCount,
            FileToken:    fileToken,
        })
        if err != nil {
            fmt.Fprintf(os.Stderr, "  WARN: manifest update for %s: %v\n", key, err)
            // Continue anyway — manifest will self-heal next upload
        }

        // Update state
        st.MarkUploaded(key, sess.MessageCount, fileToken, time.Now().UTC().Format(time.RFC3339))
        stats.Synced++
        fmt.Printf("  OK: %s (%d msgs)\n", key, sess.MessageCount)
    }

    // Save state
    if err := st.Save(); err != nil {
        fmt.Fprintf(os.Stderr, "WARN: save state: %v\n", err)
    }

    return stats, nil
}
