package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverInDir(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "claude", "projects")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	sessionPath := filepath.Join(agentDir, "abc123.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"hello"}`+"\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	results, err := discoverInDir(dir, "claude", false)
	if err != nil {
		t.Fatalf("discoverInDir: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Agent != "claude" {
		t.Errorf("agent = %q", results[0].Agent)
	}
	if results[0].SessionID != "abc123" {
		t.Errorf("sessionID = %q", results[0].SessionID)
	}
	if results[0].Size == 0 {
		t.Error("file size should be > 0")
	}
}

func TestDiscoverInDir_IgnoresNonJSONL(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644)

	results, err := discoverInDir(dir, "test-agent", false)
	if err != nil {
		t.Fatalf("discoverInDir: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestDiscoverInDir_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	hiddenDir := filepath.Join(dir, ".hidden")
	os.MkdirAll(hiddenDir, 0755)
	os.WriteFile(filepath.Join(hiddenDir, "session.jsonl"), []byte(`{}`+"\n"), 0644)

	// _synced is NOT skipped by scanner (only by upload.go's discoverFiles).
	normalDir := filepath.Join(dir, "normal")
	os.MkdirAll(normalDir, 0755)
	os.WriteFile(filepath.Join(normalDir, "visible.jsonl"), []byte(`{}`+"\n"), 0644)

	results, err := discoverInDir(dir, "test-agent", false)
	if err != nil {
		t.Fatalf("discoverInDir: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1 (only .hidden dir should be skipped)", len(results))
	}
}

func TestDiscoverInDir_MultipleSessions(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0755)

	for _, sid := range []string{"sess-a", "sess-b", "sess-c"} {
		os.WriteFile(filepath.Join(dir, sid+".jsonl"), []byte(`{}`+"\n"), 0644)
	}

	results, err := discoverInDir(dir, "claude", false)
	if err != nil {
		t.Fatalf("discoverInDir: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}
}
