package synccloud

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	sessionconflux "github.com/yanen-bohoon/session-conflux/pkg/sessionconflux"
	confluxsync "github.com/yanen-bohoon/session-conflux/pkg/sync"

	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/parser"
	"github.com/wesm/agentsview/internal/sync"
)

// nextScheduleTime parses a "HH:MM" schedule and returns the next
// occurrence. If the time has already passed today, returns tomorrow.
func nextScheduleTime(schedule string) (time.Time, error) {
	var h, m int
	if _, err := fmt.Sscanf(schedule, "%d:%d", &h, &m); err != nil {
		return time.Time{}, fmt.Errorf("invalid schedule %q: %w", schedule, err)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, fmt.Errorf("invalid schedule %q", schedule)
	}
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next, nil
}

// RunDaemon starts a background ticker that runs cloud sync at the
// configured schedule. It blocks until ctx is cancelled.
func RunDaemon(ctx context.Context, cfg *config.Config) {
	if !cfg.Sync.Enabled {
		return
	}

	next, err := nextScheduleTime(cfg.Sync.Schedule)
	if err != nil {
		log.Printf("cloud sync daemon: invalid schedule %q: %v", cfg.Sync.Schedule, err)
		return
	}

	log.Printf("cloud sync daemon: next run at %s", next.Format(time.RFC3339))

	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			runSync(cfg)
			// Reset for next day.
			next = next.Add(24 * time.Hour)
			timer.Reset(time.Until(next))
			log.Printf("cloud sync daemon: next run at %s", next.Format(time.RFC3339))
		}
	}
}

func runSync(cfg *config.Config) {
	scCfg := ToSessionConfluxConfig(&cfg.Sync)
	st, err := LoadState(cfg.DataDir)
	if err != nil {
		log.Printf("cloud sync daemon: load state: %v", err)
		return
	}

	dir := cfg.Sync.Direction

	if dir == "both" || dir == "upload" {
		log.Printf("cloud sync daemon: starting upload")
		
		engine := sync.NewEngine(nil, sync.EngineConfig{
			AgentDirs: cfg.AgentDirs,
			Machine:   "local",
		})
		discoveredFiles := engine.ChangedFiles(time.Time{})
		var files []confluxsync.SyncFile
		for _, f := range discoveredFiles {
			info, err := os.Stat(f.Path)
			if err != nil {
				continue
			}
			files = append(files, confluxsync.FileFromDiscovered(f.Path, string(f.Agent), info.Size(), info.ModTime().UnixNano()))
		}

		stats, err := sessionconflux.Upload(scCfg, st, files)
		if err != nil {
			log.Printf("cloud sync daemon: upload: %v", err)
		} else {
			log.Printf("cloud sync daemon: upload done (%d synced, %d skipped, %d failed)",
				stats.Synced, stats.Skipped, stats.Failed)
		}
	}

	if dir == "both" || dir == "download" {
		log.Printf("cloud sync daemon: starting download")
		findAgentDir := func(agent string) string {
			dirs := cfg.AgentDirs[parser.AgentType(agent)]
			if len(dirs) > 0 {
				return dirs[0]
			}
			return ""
		}
		stats, err := sessionconflux.Download(scCfg, st, findAgentDir)
		if err != nil {
			log.Printf("cloud sync daemon: download: %v", err)
		} else {
			log.Printf("cloud sync daemon: download done (%d synced)",
				stats.Synced)
		}
	}

	if err := st.Save(); err != nil {
		log.Printf("cloud sync daemon: save state: %v", err)
	}
}
