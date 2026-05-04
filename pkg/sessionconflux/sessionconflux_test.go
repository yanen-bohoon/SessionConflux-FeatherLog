package sessionconflux

import (
	"testing"
	"time"

	"github.com/yanen-bohoon/session-conflux/pkg/state"
)

func TestStatus(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	// Empty state
	info := Status(st)
	if info.Entries != 0 {
		t.Errorf("empty state: expected 0 entries, got %d", info.Entries)
	}

	// Add an entry with upload only
	key1 := "mac-studio/claude/sess-001"
	st.MarkUploaded(key1, 1024, 1000, "token1", time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))
	info = Status(st)
	if info.Entries != 1 {
		t.Errorf("expected 1 entry, got %d", info.Entries)
	}
	if info.UploadedCount != 1 {
		t.Errorf("expected 1 uploaded, got %d", info.UploadedCount)
	}
	if info.DownloadedCount != 0 {
		t.Errorf("expected 0 downloaded, got %d", info.DownloadedCount)
	}
	if info.LastUpload == "" {
		t.Error("expected last_upload to be set")
	}

	// Add another entry with download
	key2 := "thinkpad/codex/sess-002"
	st.MarkUploaded(key2, 2048, 2000, "token2", time.Date(2026, 5, 2, 14, 0, 0, 0, time.UTC))
	st.MarkDownloaded(key2, "token2", time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC))
	info = Status(st)
	if info.Entries != 2 {
		t.Errorf("expected 2 entries, got %d", info.Entries)
	}
	if info.UploadedCount != 2 {
		t.Errorf("expected 2 uploaded, got %d", info.UploadedCount)
	}
	if info.DownloadedCount != 1 {
		t.Errorf("expected 1 downloaded, got %d", info.DownloadedCount)
	}
	if info.LastDownload == "" {
		t.Error("expected last_download to be set")
	}
}

func TestStatusMostRecent(t *testing.T) {
	dir := t.TempDir()
	st, err := state.LoadFrom(dir + "/sync-state.json")
	if err != nil {
		t.Fatal(err)
	}

	// Multiple entries with different times — should pick most recent.
	st.MarkUploaded("a/claude/s1", 100, 1, "t1", time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC))
	st.MarkUploaded("b/claude/s2", 200, 2, "t2", time.Date(2026, 5, 3, 20, 0, 0, 0, time.UTC))

	info := Status(st)
	if info.LastUpload != "2026-05-03T20:00:00Z" {
		t.Errorf("expected most recent upload 2026-05-03T20:00:00Z, got %s", info.LastUpload)
	}
}
