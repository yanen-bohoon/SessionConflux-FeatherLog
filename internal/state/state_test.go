package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.All()) != 0 {
		t.Errorf("expected empty store, got %d entries", len(s.All()))
	}
}

func TestHasChanged_New(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, _ := LoadFrom(path)

	if !s.HasChanged("key1", 100, 2000) {
		t.Error("new key should report changed")
	}
	if s.HasChanged("key2", 0, 0) {
		t.Error("zero-size new entry should report unchanged")
	}
}

func TestHasChanged_Existing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, _ := LoadFrom(path)

	s.MarkUploaded("key1", 100, 2000, "ft1", "2026-01-01T00:00:00Z")
	s.Save()

	// Reload and verify no change.
	s2, _ := LoadFrom(path)
	if s2.HasChanged("key1", 100, 2000) {
		t.Error("same size/mtime should report unchanged")
	}
	if !s2.HasChanged("key1", 200, 2000) {
		t.Error("different size should report changed")
	}
	if !s2.HasChanged("key1", 100, 3000) {
		t.Error("different mtime should report changed")
	}
}

func TestMarkUploaded_And_Save(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, _ := LoadFrom(path)
	s.MarkUploaded("host/claude/sess1", 500, 6000, "file-token-1", "2026-05-01T02:00:00Z")
	s.MarkUploaded("host/codex/sess2", 800, 9000, "file-token-2", "2026-05-01T02:00:00Z")

	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reload and verify both entries persist.
	s2, _ := LoadFrom(path)
	all := s2.All()
	if len(all) != 2 {
		t.Fatalf("got %d entries, want 2", len(all))
	}

	e1, ok := s2.Get("host/claude/sess1")
	if !ok {
		t.Fatal("missing host/claude/sess1")
	}
	if e1.FileSize != 500 || e1.Mtime != 6000 {
		t.Errorf("entry1: size=%d mtime=%d", e1.FileSize, e1.Mtime)
	}
	if e1.FileToken != "file-token-1" {
		t.Errorf("file_token = %q", e1.FileToken)
	}
	if e1.LastUploaded != "2026-05-01T02:00:00Z" {
		t.Errorf("last_uploaded = %q", e1.LastUploaded)
	}
}

func TestNeedsDownload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, _ := LoadFrom(path)

	// Never downloaded — needs download.
	if !s.NeedsDownload("k", "ft1") {
		t.Error("never downloaded key should need download")
	}

	s.MarkDownloaded("k", "ft1", "2026-05-01T02:00:00Z")
	s.Save()

	// Same token — no download needed.
	if s.NeedsDownload("k", "ft1") {
		t.Error("same downloaded token should not need download")
	}

	// Different token — needs download.
	if !s.NeedsDownload("k", "ft2") {
		t.Error("different token should need download")
	}
}

func TestMarkDownloaded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, _ := LoadFrom(path)
	s.MarkDownloaded("k", "ft-dl", "2026-05-02T00:00:00Z")
	s.Save()

	s2, _ := LoadFrom(path)
	e, ok := s2.Get("k")
	if !ok {
		t.Fatal("entry not found")
	}
	if e.DownloadedToken != "ft-dl" {
		t.Errorf("downloaded_token = %q", e.DownloadedToken)
	}
	if e.LastDownloaded != "2026-05-02T00:00:00Z" {
		t.Errorf("last_downloaded = %q", e.LastDownloaded)
	}
}

func TestGet_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, _ := LoadFrom(path)

	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestSave_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "state.json")

	s, _ := LoadFrom(path)
	s.MarkUploaded("k", 10, 20, "ft", "now")
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestLoadFrom_Corrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write invalid JSON.
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("corrupted file should not error: %v", err)
	}
	if len(s.All()) != 0 {
		t.Error("corrupted file should yield empty store")
	}
}

func TestDefaultPath(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(filepath.Dir(p)) != ".session-conflux" {
		t.Errorf("parent = %q, want .session-conflux", filepath.Base(filepath.Dir(p)))
	}
	if filepath.Base(p) != "state.json" {
		t.Errorf("file = %q, want state.json", filepath.Base(p))
	}
}
