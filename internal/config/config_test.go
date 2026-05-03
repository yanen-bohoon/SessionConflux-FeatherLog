package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Sync.Schedule != "02:00" {
		t.Errorf("schedule = %q, want %q", cfg.Sync.Schedule, "02:00")
	}
	if cfg.Sync.Direction != "both" {
		t.Errorf("direction = %q, want %q", cfg.Sync.Direction, "both")
	}
	if cfg.Compression.Level != 3 {
		t.Errorf("level = %d, want 3", cfg.Compression.Level)
	}
}

func TestLoadFrom_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.toml")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return defaults when file not found.
	if cfg.Sync.Schedule != "02:00" {
		t.Errorf("schedule = %q, want default 02:00", cfg.Sync.Schedule)
	}
}

func TestSaveTo_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := DefaultConfig()
	cfg.Feishu.AppID = "test-app-id"
	cfg.Feishu.AppSecret = "test-secret"
	cfg.Feishu.FolderToken = "folder-123"
	cfg.Sync.Schedule = "03:00"
	cfg.Compression.Level = 5
	cfg.Agents.Exclude = []string{"warp", "copilot"}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Load back and verify.
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Feishu.AppID != "test-app-id" {
		t.Errorf("app_id = %q", loaded.Feishu.AppID)
	}
	if loaded.Feishu.AppSecret != "test-secret" {
		t.Errorf("app_secret = %q", loaded.Feishu.AppSecret)
	}
	if loaded.Feishu.FolderToken != "folder-123" {
		t.Errorf("folder_token = %q", loaded.Feishu.FolderToken)
	}
	if loaded.Sync.Schedule != "03:00" {
		t.Errorf("schedule = %q", loaded.Sync.Schedule)
	}
	if loaded.Compression.Level != 5 {
		t.Errorf("level = %d", loaded.Compression.Level)
	}
	if len(loaded.Agents.Exclude) != 2 {
		t.Errorf("exclude len = %d, want 2", len(loaded.Agents.Exclude))
	}
}

func TestSaveTo_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.toml")

	cfg := DefaultConfig()
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestDefaultPath(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Path should end in .session-conflux/config.toml.
	if filepath.Base(filepath.Dir(p)) != ".session-conflux" {
		t.Errorf("parent dir = %q, want .session-conflux", filepath.Base(filepath.Dir(p)))
	}
	if filepath.Base(p) != "config.toml" {
		t.Errorf("file = %q, want config.toml", filepath.Base(p))
	}
}

func TestSaveTo_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg1 := DefaultConfig()
	cfg1.Feishu.AppID = "first"
	if err := cfg1.SaveTo(path); err != nil {
		t.Fatalf("first save: %v", err)
	}

	cfg2 := DefaultConfig()
	cfg2.Feishu.AppID = "second"
	if err := cfg2.SaveTo(path); err != nil {
		t.Fatalf("second save: %v", err)
	}

	loaded, _ := LoadFrom(path)
	if loaded.Feishu.AppID != "second" {
		t.Errorf("app_id = %q, want second", loaded.Feishu.AppID)
	}
}
