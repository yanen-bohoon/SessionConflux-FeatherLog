// Package synccloud bridges agentsview config/state with the
// session-conflux public API (pkg/sessionconflux).
package synccloud

import (
	"fmt"
	"os"
	"path/filepath"

	scconfig "github.com/yanen-bohoon/session-conflux/pkg/config"
	"github.com/yanen-bohoon/session-conflux/pkg/state"

	"github.com/wesm/agentsview/internal/config"
)

// ToSessionConfluxConfig maps an agentsview SyncConfig to a session-conflux Config.
// Transport types are unified, so the transport field copies directly.
func ToSessionConfluxConfig(sc *config.SyncConfig) *scconfig.Config {
	return &scconfig.Config{
		Transport: sc.Transport,
		Sync: scconfig.SyncConfig{
			Schedule:  sc.Schedule,
			Direction: sc.Direction,
		},
		Agents: scconfig.AgentsConfig{
			Exclude: sc.ExcludeAgents,
		},
		Compression: scconfig.CompressionConfig{
			Level: sc.CompressionLevel,
		},
	}
}

// StatePath returns the path to the sync state file within the data directory.
func StatePath(dataDir string) string {
	return filepath.Join(dataDir, "sync-state.json")
}

// LoadState loads the sync state from the agentsview data directory.
func LoadState(dataDir string) (*state.Store, error) {
	return state.LoadFrom(StatePath(dataDir))
}

// MigrateState copies the legacy ~/.session-conflux/state.json to the
// agentsview data directory if the legacy file exists and the new one
// does not. The old file is renamed to state.json.bak.
func MigrateState(dataDir string) error {
	oldPath, err := state.DefaultPath()
	if err != nil {
		return nil // can't determine path, skip
	}
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil
	}

	newPath := StatePath(dataDir)
	if _, err := os.Stat(newPath); err == nil {
		return nil // already migrated
	}

	// Read old state.
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("reading old state: %w", err)
	}

	// Write to new location.
	if err := os.MkdirAll(filepath.Dir(newPath), 0700); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}
	if err := os.WriteFile(newPath, data, 0644); err != nil {
		return fmt.Errorf("writing new state: %w", err)
	}

	// Rename old file.
	if err := os.Rename(oldPath, oldPath+".bak"); err != nil {
		return fmt.Errorf("renaming old state: %w", err)
	}

	return nil
}
