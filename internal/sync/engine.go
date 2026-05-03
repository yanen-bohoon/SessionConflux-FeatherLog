package sync

import (
	"fmt"

	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/state"
)

// RunFullSync performs a bidirectional sync (upload then download).
func RunFullSync(cfg *config.Config) error {
	client := feishu.NewClient(cfg.Feishu.AppID, cfg.Feishu.AppSecret)

	st, err := state.Load()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	direction := cfg.Sync.Direction
	if direction == "" {
		direction = "both"
	}

	switch direction {
	case "upload":
		fmt.Println("--- Upload ---")
		stats, err := UploadChanged(client, cfg, st)
		if err != nil {
			return fmt.Errorf("upload: %w", err)
		}
		fmt.Printf("Upload: %d total, %d synced, %d skipped, %d failed.\n",
			stats.Total, stats.Synced, stats.Skipped, stats.Failed)

	case "download":
		fmt.Println("--- Download ---")
		n, err := DownloadAllSessions(client, cfg)
		if err != nil {
			return fmt.Errorf("download: %w", err)
		}
		fmt.Printf("Download: %d sessions downloaded.\n", n)

	case "both":
		fmt.Println("--- Upload ---")
		stats, err := UploadChanged(client, cfg, st)
		if err != nil {
			return fmt.Errorf("upload: %w", err)
		}
		fmt.Printf("Upload: %d total, %d synced, %d skipped, %d failed.\n",
			stats.Total, stats.Synced, stats.Skipped, stats.Failed)

		fmt.Println("\n--- Download ---")
		n, err := DownloadAllSessions(client, cfg)
		if err != nil {
			return fmt.Errorf("download: %w", err)
		}
		fmt.Printf("Download: %d sessions downloaded.\n", n)
	}
	return nil
}
