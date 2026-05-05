// ABOUTME: CLI subcommand that syncs session data into the database
// ABOUTME: without starting the HTTP server.
package avcli

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/parser"
	"github.com/wesm/agentsview/internal/ssh"
	"github.com/wesm/agentsview/internal/sync"
)

// SyncConfig holds parsed CLI options for the sync command.
type SyncConfig struct {
	Full bool
	Host string
	User string
	Port int
}

func RunSync(cfg SyncConfig) {
	appCfg, err := config.LoadMinimal()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	if err := os.MkdirAll(appCfg.DataDir, 0o755); err != nil {
		log.Fatalf("creating data dir: %v", err)
	}

	SetupLogFile(appCfg.DataDir)

	ApplyClassifierConfig(appCfg)
	database, err := db.Open(appCfg.DBPath)
	if err != nil {
		Fatal("opening database: %v", err)
	}
	defer database.Close()

	if appCfg.CursorSecret != "" {
		secret, decErr := base64.StdEncoding.DecodeString(
			appCfg.CursorSecret,
		)
		if decErr != nil {
			Fatal("invalid cursor secret: %v", decErr)
		}
		database.SetCursorSecret(secret)
	}

	if cfg.Host != "" {
		RunRemoteSync(appCfg, database, cfg)
		return
	}

	RunLocalSync(appCfg, database, cfg.Full)
}

func RunRemoteSync(
	appCfg config.Config, database *db.DB, cfg SyncConfig,
) {
	rs := &ssh.RemoteSync{
		Host:                    cfg.Host,
		User:                    cfg.User,
		Port:                    cfg.Port,
		Full:                    cfg.Full,
		DB:                      database,
		BlockedResultCategories: appCfg.ResultContentBlockedCategories,
	}
	ctx := context.Background()
	if _, err := rs.Run(ctx); err != nil {
		Fatal("remote sync: %v", err)
	}
}

// RunLocalSync runs a local sync (incremental or full resync).
// It returns true if a full resync was performed, which callers
// can use to force a full PG push (watermarks become stale after
// a local resync).
func RunLocalSync(
	appCfg config.Config, database *db.DB, full bool,
) bool {
	for _, def := range parser.Registry {
		if !appCfg.IsUserConfigured(def.Type) {
			continue
		}
		WarnMissingDirs(
			appCfg.ResolveDirs(def.Type),
			string(def.Type),
		)
	}

	CleanResyncTemp(appCfg.DBPath)

	engine := sync.NewEngine(database, sync.EngineConfig{
		AgentDirs: appCfg.AgentDirs,
		Machine:   "local",
	})

	didResync := full || database.NeedsResync()
	ctx := context.Background()
	if didResync {
		RunInitialResync(ctx, engine)
	} else {
		RunInitialSync(ctx, engine)
	}

	fmt.Println()
	stats, err := database.GetStats(
		context.Background(), false, false,
	)
	if err == nil {
		fmt.Printf(
			"Database: %d sessions, %d messages\n",
			stats.SessionCount, stats.MessageCount,
		)
	}
	return didResync
}

func valueOrNever(s string) string {
	if s == "" {
		return "never"
	}
	return s
}
