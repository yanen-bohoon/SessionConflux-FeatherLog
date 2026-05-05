package synccloud

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	scconfig "github.com/yanen-bohoon/session-conflux/pkg/config"
	"github.com/yanen-bohoon/session-conflux/pkg/state"

	"github.com/wesm/agentsview/internal/config"
)

// --- ToSessionConfluxConfig --------------------------------------------------------

func TestToSessionConfluxConfig_Feishu(t *testing.T) {
	sc := &config.SyncConfig{
		Enabled:   true,
		Schedule:  "03:00",
		Direction: "upload",
		Transport: scconfig.TransportConfig{
			Backend: "feishu",
			Feishu: scconfig.FeishuConfig{
				AppID:       "app-123",
				AppSecret:   "secret-abc",
				FolderToken: "folder-xyz",
			},
		},
		ExcludeAgents:    []string{"agent-a", "agent-b"},
		CompressionLevel: 7,
	}

	got := ToSessionConfluxConfig(sc)

	if got.Transport.Backend != "feishu" {
		t.Errorf("Transport.Backend = %q, want feishu", got.Transport.Backend)
	}
	if got.Transport.Feishu.AppID != "app-123" {
		t.Errorf("Transport.Feishu.AppID = %q, want app-123", got.Transport.Feishu.AppID)
	}
	if got.Transport.Feishu.AppSecret != "secret-abc" {
		t.Errorf("Transport.Feishu.AppSecret = %q, want secret-abc", got.Transport.Feishu.AppSecret)
	}
	if got.Transport.Feishu.FolderToken != "folder-xyz" {
		t.Errorf("Transport.Feishu.FolderToken = %q, want folder-xyz", got.Transport.Feishu.FolderToken)
	}
	if got.Sync.Schedule != "03:00" {
		t.Errorf("Sync.Schedule = %q, want 03:00", got.Sync.Schedule)
	}
	if got.Sync.Direction != "upload" {
		t.Errorf("Sync.Direction = %q, want upload", got.Sync.Direction)
	}
	if len(got.Agents.Exclude) != 2 || got.Agents.Exclude[0] != "agent-a" || got.Agents.Exclude[1] != "agent-b" {
		t.Errorf("Agents.Exclude = %v, want [agent-a agent-b]", got.Agents.Exclude)
	}
	if got.Compression.Level != 7 {
		t.Errorf("Compression.Level = %d, want 7", got.Compression.Level)
	}
}

func TestToSessionConfluxConfig_SSH(t *testing.T) {
	sc := &config.SyncConfig{
		Enabled:   true,
		Schedule:  "04:00",
		Direction: "download",
		Transport: scconfig.TransportConfig{
			Backend: "ssh",
			SSH: scconfig.SSHConfig{
				Host:       "example.com",
				Port:       2222,
				User:       "deploy",
				KeyFile:    "/home/deploy/.ssh/id_ed25519",
				RemotePath: "/srv/sessions",
			},
		},
		CompressionLevel: 12,
	}

	got := ToSessionConfluxConfig(sc)

	if got.Transport.Backend != "ssh" {
		t.Errorf("Transport.Backend = %q, want ssh", got.Transport.Backend)
	}
	if got.Transport.SSH.Host != "example.com" {
		t.Errorf("Transport.SSH.Host = %q, want example.com", got.Transport.SSH.Host)
	}
	if got.Transport.SSH.Port != 2222 {
		t.Errorf("Transport.SSH.Port = %d, want 2222", got.Transport.SSH.Port)
	}
	if got.Transport.SSH.User != "deploy" {
		t.Errorf("Transport.SSH.User = %q, want deploy", got.Transport.SSH.User)
	}
	if got.Transport.SSH.KeyFile != "/home/deploy/.ssh/id_ed25519" {
		t.Errorf("Transport.SSH.KeyFile = %q, want /home/deploy/.ssh/id_ed25519", got.Transport.SSH.KeyFile)
	}
	if got.Transport.SSH.RemotePath != "/srv/sessions" {
		t.Errorf("Transport.SSH.RemotePath = %q, want /srv/sessions", got.Transport.SSH.RemotePath)
	}
	if got.Sync.Schedule != "04:00" {
		t.Errorf("Sync.Schedule = %q, want 04:00", got.Sync.Schedule)
	}
	if got.Sync.Direction != "download" {
		t.Errorf("Sync.Direction = %q, want download", got.Sync.Direction)
	}
	if got.Compression.Level != 12 {
		t.Errorf("Compression.Level = %d, want 12", got.Compression.Level)
	}
}

func TestToSessionConfluxConfig_Minimal(t *testing.T) {
	sc := &config.SyncConfig{
		Enabled: false,
	}

	got := ToSessionConfluxConfig(sc)

	if got.Transport.Backend != "" {
		t.Errorf("Transport.Backend = %q, want empty", got.Transport.Backend)
	}
	if got.Sync.Schedule != "" {
		t.Errorf("Sync.Schedule = %q, want empty", got.Sync.Schedule)
	}
	if got.Sync.Direction != "" {
		t.Errorf("Sync.Direction = %q, want empty", got.Sync.Direction)
	}
	if len(got.Agents.Exclude) != 0 {
		t.Errorf("Agents.Exclude = %v, want empty", got.Agents.Exclude)
	}
	if got.Compression.Level != 0 {
		t.Errorf("Compression.Level = %d, want 0", got.Compression.Level)
	}
}

func TestToSessionConfluxConfig_AllAgentsExcluded(t *testing.T) {
	sc := &config.SyncConfig{
		ExcludeAgents: []string{"claude", "codex", "cursor", "windsurf", "aider", "kilocode", "auggie"},
	}

	got := ToSessionConfluxConfig(sc)

	if len(got.Agents.Exclude) != 7 {
		t.Errorf("Agents.Exclude has %d entries, want 7", len(got.Agents.Exclude))
	}
	expected := []string{"claude", "codex", "cursor", "windsurf", "aider", "kilocode", "auggie"}
	for i, v := range expected {
		if got.Agents.Exclude[i] != v {
			t.Errorf("Agents.Exclude[%d] = %q, want %q", i, got.Agents.Exclude[i], v)
		}
	}
}

func TestToSessionConfluxConfig_CompressionLevels(t *testing.T) {
	tests := []struct {
		name  string
		level int
	}{
		{"zero (default)", 0},
		{"minimum (1)", 1},
		{"default (3)", 3},
		{"mid-range (11)", 11},
		{"maximum (22)", 22},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &config.SyncConfig{
				CompressionLevel: tt.level,
			}
			got := ToSessionConfluxConfig(sc)
			if got.Compression.Level != tt.level {
				t.Errorf("Compression.Level = %d, want %d", got.Compression.Level, tt.level)
			}
		})
	}
}

func TestToSessionConfluxConfig_Directions(t *testing.T) {
	tests := []struct {
		name      string
		direction string
	}{
		{"both", "both"},
		{"upload", "upload"},
		{"download", "download"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &config.SyncConfig{
				Direction: tt.direction,
			}
			got := ToSessionConfluxConfig(sc)
			if got.Sync.Direction != tt.direction {
				t.Errorf("Sync.Direction = %q, want %q", got.Sync.Direction, tt.direction)
			}
		})
	}
}

func TestToSessionConfluxConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		sc    *config.SyncConfig
		check func(t *testing.T, got *scconfig.Config)
	}{
		{
			name: "full feishu config",
			sc: &config.SyncConfig{
				Schedule:  "02:00",
				Direction: "both",
				Transport: scconfig.TransportConfig{
					Backend: "feishu",
					Feishu:  scconfig.FeishuConfig{AppID: "aid", AppSecret: "sec"},
				},
				ExcludeAgents:    []string{"aider"},
				CompressionLevel: 5,
			},
			check: func(t *testing.T, got *scconfig.Config) {
				if got.Sync.Schedule != "02:00" {
					t.Errorf("schedule: got %q", got.Sync.Schedule)
				}
				if got.Sync.Direction != "both" {
					t.Errorf("direction: got %q", got.Sync.Direction)
				}
				if got.Transport.Backend != "feishu" {
					t.Errorf("backend: got %q", got.Transport.Backend)
				}
				if got.Compression.Level != 5 {
					t.Errorf("compression: got %d", got.Compression.Level)
				}
				if len(got.Agents.Exclude) != 1 || got.Agents.Exclude[0] != "aider" {
					t.Errorf("exclude: got %v", got.Agents.Exclude)
				}
			},
		},
		{
			name: "full ssh config",
			sc: &config.SyncConfig{
				Schedule:  "06:00",
				Direction: "upload",
				Transport: scconfig.TransportConfig{
					Backend: "ssh",
					SSH:     scconfig.SSHConfig{Host: "srv.local", Port: 22, User: "root"},
				},
				CompressionLevel: 3,
			},
			check: func(t *testing.T, got *scconfig.Config) {
				if got.Transport.Backend != "ssh" {
					t.Errorf("backend: got %q", got.Transport.Backend)
				}
				if got.Transport.SSH.Host != "srv.local" {
					t.Errorf("ssh host: got %q", got.Transport.SSH.Host)
				}
				if got.Sync.Direction != "upload" {
					t.Errorf("direction: got %q", got.Sync.Direction)
				}
			},
		},
		{
			name: "empty sync config (nil transport fields)",
			sc:   &config.SyncConfig{},
			check: func(t *testing.T, got *scconfig.Config) {
				if got.Transport.Backend != "" {
					t.Errorf("backend: got %q, want empty", got.Transport.Backend)
				}
				if got.Sync.Schedule != "" {
					t.Errorf("schedule: got %q, want empty", got.Sync.Schedule)
				}
				if got.Compression.Level != 0 {
					t.Errorf("compression: got %d, want 0", got.Compression.Level)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToSessionConfluxConfig(tt.sc)
			tt.check(t, got)
		})
	}
}

// --- LoadState ---------------------------------------------------------------------

func TestLoadState_MissingFile(t *testing.T) {
	dir := t.TempDir()

	st, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if st == nil {
		t.Fatal("LoadState returned nil store")
	}
	if len(st.All()) != 0 {
		t.Errorf("expected 0 entries, got %d", len(st.All()))
	}
}

func TestLoadState_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "sync-state.json")
	if err := os.WriteFile(stateFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	st, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if st == nil {
		t.Fatal("LoadState returned nil store")
	}
	if len(st.All()) != 0 {
		t.Errorf("expected 0 entries, got %d", len(st.All()))
	}
}

func TestLoadState_Populated(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "sync-state.json")

	data := map[string]state.Entry{
		"mac-studio/claude/sess-001": {
			FileSize:     1024,
			Mtime:        1000,
			FileToken:    "upload-token-1",
			LastUploaded: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
		"thinkpad/codex/sess-002": {
			FileSize:        2048,
			Mtime:           2000,
			FileToken:       "upload-token-2",
			LastUploaded:    time.Date(2026, 5, 2, 14, 0, 0, 0, time.UTC),
			DownloadedToken: "download-token-2",
			LastDownloaded:  time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC),
		},
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	st, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if st == nil {
		t.Fatal("LoadState returned nil store")
	}

	all := st.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	e1, ok := all["mac-studio/claude/sess-001"]
	if !ok {
		t.Fatal("missing entry mac-studio/claude/sess-001")
	}
	if e1.FileSize != 1024 {
		t.Errorf("FileSize = %d, want 1024", e1.FileSize)
	}
	if e1.FileToken != "upload-token-1" {
		t.Errorf("FileToken = %q, want upload-token-1", e1.FileToken)
	}

	e2, ok := all["thinkpad/codex/sess-002"]
	if !ok {
		t.Fatal("missing entry thinkpad/codex/sess-002")
	}
	if e2.DownloadedToken != "download-token-2" {
		t.Errorf("DownloadedToken = %q, want download-token-2", e2.DownloadedToken)
	}
	if e2.LastDownloaded.IsZero() {
		t.Error("LastDownloaded is zero, expected non-zero")
	}
}

func TestLoadState_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "sync-state.json")
	if err := os.WriteFile(stateFile, []byte("this is not valid json {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	st, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if st == nil {
		t.Fatal("LoadState returned nil store")
	}
	// Malformed file should result in an empty store (graceful recovery).
	if len(st.All()) != 0 {
		t.Errorf("expected 0 entries after malformed file, got %d", len(st.All()))
	}
}

func TestLoadState_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "sync-state.json")

	// Write a state file with 50 entries.
	data := make(map[string]state.Entry, 50)
	for i := range 50 {
		key := "machine-" + string(rune('A'+i%26)) + "/agent/sess-" + string(rune('0'+i%10))
		data[key] = state.Entry{
			FileSize:     int64(i * 100),
			Mtime:        int64(i * 1000),
			FileToken:    "token",
			LastUploaded: time.Date(2026, 5, 1, 0, 0, i, 0, time.UTC),
		}
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, encoded, 0644); err != nil {
		t.Fatal(err)
	}

	st, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if len(st.All()) != 50 {
		t.Errorf("expected 50 entries, got %d", len(st.All()))
	}
}

// --- Status ------------------------------------------------------------------------

func TestStatus_Empty(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	info := Status(st)
	if info.Entries != 0 {
		t.Errorf("Entries = %d, want 0", info.Entries)
	}
	if info.UploadedCount != 0 {
		t.Errorf("UploadedCount = %d, want 0", info.UploadedCount)
	}
	if info.DownloadedCount != 0 {
		t.Errorf("DownloadedCount = %d, want 0", info.DownloadedCount)
	}
	if info.LastUpload != "" {
		t.Errorf("LastUpload = %q, want empty", info.LastUpload)
	}
	if info.LastDownload != "" {
		t.Errorf("LastDownload = %q, want empty", info.LastDownload)
	}
}

func TestStatus_UploadOnly(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	st.MarkUploaded("a/claude/s1", 100, 1, "t1", time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC))
	st.MarkUploaded("b/claude/s2", 200, 2, "t2", time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))

	info := Status(st)
	if info.Entries != 2 {
		t.Errorf("Entries = %d, want 2", info.Entries)
	}
	if info.UploadedCount != 2 {
		t.Errorf("UploadedCount = %d, want 2", info.UploadedCount)
	}
	if info.DownloadedCount != 0 {
		t.Errorf("DownloadedCount = %d, want 0", info.DownloadedCount)
	}
	if info.LastUpload == "" {
		t.Error("LastUpload should be set")
	}
	if info.LastDownload != "" {
		t.Errorf("LastDownload = %q, want empty", info.LastDownload)
	}
}

func TestStatus_DownloadOnly(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	// MarkDownloaded doesn't set LastUploaded, so these are download-only entries.
	st.MarkDownloaded("a/claude/s1", "dt1", time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC))
	st.MarkDownloaded("b/claude/s2", "dt2", time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))

	info := Status(st)
	if info.Entries != 2 {
		t.Errorf("Entries = %d, want 2", info.Entries)
	}
	if info.UploadedCount != 0 {
		t.Errorf("UploadedCount = %d, want 0", info.UploadedCount)
	}
	if info.DownloadedCount != 2 {
		t.Errorf("DownloadedCount = %d, want 2", info.DownloadedCount)
	}
	if info.LastUpload != "" {
		t.Errorf("LastUpload = %q, want empty", info.LastUpload)
	}
	if info.LastDownload == "" {
		t.Error("LastDownload should be set")
	}
}

func TestStatus_Mixed(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	// Mix of upload+download.
	st.MarkUploaded("a/claude/s1", 100, 1, "t1", time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC))
	st.MarkDownloaded("a/claude/s1", "dt1", time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC))
	st.MarkUploaded("b/claude/s2", 200, 2, "t2", time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC))
	// s2 has upload but no download.

	info := Status(st)
	if info.Entries != 2 {
		t.Errorf("Entries = %d, want 2", info.Entries)
	}
	if info.UploadedCount != 2 {
		t.Errorf("UploadedCount = %d, want 2", info.UploadedCount)
	}
	if info.DownloadedCount != 1 {
		t.Errorf("DownloadedCount = %d, want 1", info.DownloadedCount)
	}
}

func TestStatus_MostRecentUpload(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	st.MarkUploaded("a/claude/s1", 100, 1, "t1", time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC))
	st.MarkUploaded("b/claude/s2", 200, 2, "t2", time.Date(2026, 5, 3, 20, 0, 0, 0, time.UTC))
	st.MarkUploaded("c/claude/s3", 300, 3, "t3", time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC))

	info := Status(st)
	if info.LastUpload != "2026-05-03T20:00:00Z" {
		t.Errorf("LastUpload = %q, want 2026-05-03T20:00:00Z", info.LastUpload)
	}
}

func TestStatus_MostRecentDownload(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	st.MarkDownloaded("a/claude/s1", "dt1", time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC))
	st.MarkDownloaded("b/claude/s2", "dt2", time.Date(2026, 5, 4, 22, 0, 0, 0, time.UTC))
	st.MarkDownloaded("c/claude/s3", "dt3", time.Date(2026, 5, 3, 15, 30, 0, 0, time.UTC))

	info := Status(st)
	if info.LastDownload != "2026-05-04T22:00:00Z" {
		t.Errorf("LastDownload = %q, want 2026-05-04T22:00:00Z", info.LastDownload)
	}
}

func TestStatus_FormatConsistency(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	// Verify the timestamp format is RFC3339-like (2006-01-02T15:04:05Z).
	ts := time.Date(2026, 5, 5, 14, 30, 0, 0, time.UTC)
	st.MarkUploaded("a/claude/s1", 100, 1, "t1", ts)

	info := Status(st)
	if info.LastUpload != "2026-05-05T14:30:00Z" {
		t.Errorf("LastUpload = %q, want 2026-05-05T14:30:00Z", info.LastUpload)
	}
}
