// Package sessionconflux provides the coarse public API for
// cross-machine session sync, designed to be embedded in agentsview.
package sessionconflux

import (
	"github.com/yanen-bohoon/session-conflux/pkg/config"
	"github.com/yanen-bohoon/session-conflux/pkg/state"
	"github.com/yanen-bohoon/session-conflux/pkg/sync"
	"github.com/yanen-bohoon/session-conflux/pkg/transport"
)

// Stats holds aggregate counts from an upload or download operation.
type Stats struct {
	Total   int `json:"total"`
	Synced  int `json:"synced"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// Info holds the current sync status for display.
type Info struct {
	Entries         int    `json:"entries"`
	UploadedCount   int    `json:"uploaded_count"`
	DownloadedCount int    `json:"downloaded_count"`
	LastUpload      string `json:"last_upload,omitempty"`
	LastDownload    string `json:"last_download,omitempty"`
}

// Upload scans local agent directories, compares against sync state,
// and uploads changed sessions to the configured transport.
func Upload(cfg *config.Config, st *state.Store, files []sync.SyncFile) (*Stats, error) {
	t, err := transport.New(cfg)
	if err != nil {
		return nil, err
	}
	result, err := sync.UploadChanged(t, cfg, st, files)
	if err != nil {
		return nil, err
	}
	return &Stats{
		Total:   result.Total,
		Synced:  result.Synced,
		Skipped: result.Skipped,
		Failed:  result.Failed,
	}, nil
}

// Download retrieves all remote sessions (from other machines)
// and writes them to local agent directories.
func Download(cfg *config.Config, st *state.Store, findAgentDir func(string) string) (*Stats, error) {
	t, err := transport.New(cfg)
	if err != nil {
		return nil, err
	}
	// DownloadAllSessions internally uses its own state loading —
	// we pass the caller's state by replacing the default path.
	// For now, DownloadAllSessions manages state internally via state.Load().
	n, err := sync.DownloadAllSessions(t, findAgentDir)
	if err != nil {
		return nil, err
	}
	return &Stats{
		Synced: n,
	}, nil
}

// Status returns the current sync state summary.
func Status(st *state.Store) *Info {
	entries := st.All()
	info := &Info{Entries: len(entries)}

	for _, e := range entries {
		if !e.LastUploaded.IsZero() {
			info.UploadedCount++
			if info.LastUpload == "" || e.LastUploaded.Format("2006-01-02T15:04:05Z") > info.LastUpload {
				info.LastUpload = e.LastUploaded.Format("2006-01-02T15:04:05Z")
			}
		}
		if !e.LastDownloaded.IsZero() {
			info.DownloadedCount++
			if info.LastDownload == "" || e.LastDownloaded.Format("2006-01-02T15:04:05Z") > info.LastDownload {
				info.LastDownload = e.LastDownloaded.Format("2006-01-02T15:04:05Z")
			}
		}
	}
	return info
}

// VerifyTransport validates transport credentials without persisting.
func VerifyTransport(cfg *config.Config) error {
	t, err := transport.New(cfg)
	if err != nil {
		return err
	}
	return t.Verify()
}
