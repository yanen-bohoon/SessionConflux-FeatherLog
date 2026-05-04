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
	cfg.Transport.Feishu.AppID = "test-app-id"
	cfg.Transport.Feishu.AppSecret = "test-secret"
	cfg.Transport.Feishu.FolderToken = "folder-123"
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
	if loaded.Transport.Feishu.AppID != "test-app-id" {
		t.Errorf("app_id = %q", loaded.Transport.Feishu.AppID)
	}
	if loaded.Transport.Feishu.AppSecret != "test-secret" {
		t.Errorf("app_secret = %q", loaded.Transport.Feishu.AppSecret)
	}
	if loaded.Transport.Feishu.FolderToken != "folder-123" {
		t.Errorf("folder_token = %q", loaded.Transport.Feishu.FolderToken)
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

func TestLoadFrom_OldFormatMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write old-style [feishu] config.
	content := `
[feishu]
app_id = "old-app-id"
app_secret = "old-secret"
folder_token = "old-folder"

[sync]
schedule = "03:00"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Transport.Backend != "feishu" {
		t.Errorf("backend = %q, want feishu", cfg.Transport.Backend)
	}
	if cfg.Transport.Feishu.AppID != "old-app-id" {
		t.Errorf("app_id = %q, want old-app-id", cfg.Transport.Feishu.AppID)
	}
	if cfg.Transport.Feishu.AppSecret != "old-secret" {
		t.Errorf("app_secret = %q, want old-secret", cfg.Transport.Feishu.AppSecret)
	}
	if cfg.Transport.Feishu.FolderToken != "old-folder" {
		t.Errorf("folder_token = %q, want old-folder", cfg.Transport.Feishu.FolderToken)
	}
}

func TestLoadFrom_NewFormatTransport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[transport]
backend = "ssh"

[transport.feishu]
app_id = "unused"

[transport.ssh]
host = "10.0.0.1"
port = 2222
user = "testuser"
key_file = "~/.ssh/test_key"
remote_path = "/data/conflux"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Transport.Backend != "ssh" {
		t.Errorf("backend = %q, want ssh", cfg.Transport.Backend)
	}
	if cfg.Transport.SSH.Host != "10.0.0.1" {
		t.Errorf("host = %q, want 10.0.0.1", cfg.Transport.SSH.Host)
	}
	if cfg.Transport.SSH.Port != 2222 {
		t.Errorf("port = %d, want 2222", cfg.Transport.SSH.Port)
	}
	if cfg.Transport.SSH.User != "testuser" {
		t.Errorf("user = %q, want testuser", cfg.Transport.SSH.User)
	}
	if cfg.Transport.SSH.KeyFile != "~/.ssh/test_key" {
		t.Errorf("key_file = %q", cfg.Transport.SSH.KeyFile)
	}
	if cfg.Transport.SSH.RemotePath != "/data/conflux" {
		t.Errorf("remote_path = %q", cfg.Transport.SSH.RemotePath)
	}
}

func TestLoadFrom_NewFormatFeishu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[transport]
backend = "feishu"

[transport.feishu]
app_id = "new-app"
app_secret = "new-secret"
folder_token = "my-folder"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Transport.Backend != "feishu" {
		t.Errorf("backend = %q, want feishu", cfg.Transport.Backend)
	}
	if cfg.Transport.Feishu.AppID != "new-app" {
		t.Errorf("app_id = %q, want new-app", cfg.Transport.Feishu.AppID)
	}
}

func TestSaveTo_TransportConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := DefaultConfig()
	cfg.Transport.Backend = "ssh"
	cfg.Transport.SSH.Host = "192.168.1.1"
	cfg.Transport.SSH.Port = 22
	cfg.Transport.SSH.User = "dev"
	cfg.Transport.SSH.KeyFile = "~/.ssh/id_rsa"
	cfg.Transport.SSH.RemotePath = "/data/sessions"

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Transport.Backend != "ssh" {
		t.Errorf("backend = %q, want ssh", loaded.Transport.Backend)
	}
	if loaded.Transport.SSH.Host != "192.168.1.1" {
		t.Errorf("host = %q", loaded.Transport.SSH.Host)
	}
}

func TestSaveTo_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg1 := DefaultConfig()
	cfg1.Transport.Feishu.AppID = "first"
	if err := cfg1.SaveTo(path); err != nil {
		t.Fatalf("first save: %v", err)
	}

	cfg2 := DefaultConfig()
	cfg2.Transport.Feishu.AppID = "second"
	if err := cfg2.SaveTo(path); err != nil {
		t.Fatalf("second save: %v", err)
	}

	loaded, _ := LoadFrom(path)
	if loaded.Transport.Feishu.AppID != "second" {
		t.Errorf("app_id = %q, want second", loaded.Transport.Feishu.AppID)
	}
}
