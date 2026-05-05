package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yanen-bohoon/session-conflux/pkg/bundle"
	"github.com/yanen-bohoon/session-conflux/pkg/compress"
	"github.com/yanen-bohoon/session-conflux/pkg/config"
	"github.com/yanen-bohoon/session-conflux/pkg/feishu"
	"github.com/yanen-bohoon/session-conflux/pkg/state"
	"github.com/yanen-bohoon/session-conflux/pkg/transport"
)

// newTestTransport creates a FeishuTransport pointed at a test server.
func newTestTransport(srv *httptest.Server, rootToken string) transport.Transport {
	c := feishu.NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.SetHTTPClient(srv.Client())
	return transport.NewFeishuTransportWithClient(c, rootToken)
}

func TestListRemoteSessions_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files":    []map[string]any{},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	tr := newTestTransport(srv, "l1-token")
	sessions, err := ListRemoteSessions(tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessions))
	}
}

func TestListRemoteSessions_WithSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}

		if strings.Contains(r.URL.String(), "folder_token=l1-token") && !strings.Contains(r.URL.String(), "page_token=") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"files": []map[string]any{
						{"token": "host1-token", "name": "mac-studio", "type": "folder"},
						{"token": "host2-token", "name": "thinkpad", "type": "folder"},
					},
					"has_more": false,
				},
			})
			return
		}
		if strings.Contains(r.URL.String(), "folder_token=host1-token") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"files": []map[string]any{
						{"token": "incr1", "name": "incremental", "type": "folder"},
					},
					"has_more": false,
				},
			})
			return
		}
		if strings.Contains(r.URL.String(), "folder_token=incr1") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"files": []map[string]any{
						{"token": "ft1", "name": "claude/sess-abc.jsonl.zst", "type": "file"},
					},
					"has_more": false,
				},
			})
			return
		}
		if strings.Contains(r.URL.String(), "folder_token=host2-token") {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"files":    []map[string]any{},
					"has_more": false,
				},
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"files": []map[string]any{}, "has_more": false},
		})
	}))
	defer srv.Close()

	tr := newTestTransport(srv, "l1-token")
	sessions, err := ListRemoteSessions(tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Key != "mac-studio/claude/sess-abc" {
		t.Errorf("key = %q, want %q", sessions[0].Key, "mac-studio/claude/sess-abc")
	}
	if sessions[0].Agent != "claude" {
		t.Errorf("agent = %q, want %q", sessions[0].Agent, "claude")
	}
	if sessions[0].Computer != "mac-studio" {
		t.Errorf("computer = %q, want %q", sessions[0].Computer, "mac-studio")
	}
}

func TestParseBundleEntry(t *testing.T) {
	host, agent, sid := parseBundleEntry("mac-studio/claude/sess-123.jsonl")
	if host != "mac-studio" {
		t.Errorf("host = %q", host)
	}
	if agent != "claude" {
		t.Errorf("agent = %q", agent)
	}
	if sid != "sess-123" {
		t.Errorf("sessionID = %q", sid)
	}
}

func TestParseBundleEntry_NestedSessionID(t *testing.T) {
	host, agent, sid := parseBundleEntry("laptop/codex/sub/dir/session.jsonl")
	if host != "laptop" {
		t.Errorf("host = %q", host)
	}
	if agent != "codex" {
		t.Errorf("agent = %q", agent)
	}
	if sid != "session" {
		t.Errorf("sessionID = %q, want 'session'", sid)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.n)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestWriteToAgentDir(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "claude", "projects")

	err := writeToAgentDir("mac-studio", "claude", "sess-123", []byte(`{"test":true}`+"\n"), agentDir)
	if err != nil {
		t.Fatalf("writeToAgentDir: %v", err)
	}

	expectedPath := filepath.Join(agentDir, "_synced", "mac-studio", "sess-123.jsonl")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(data) != `{"test":true}`+"\n" {
		t.Errorf("content = %q", string(data))
	}
}

func TestWriteToAgentDir_MultipleHosts(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "codex", "sessions")

	writeToAgentDir("host-a", "codex", "s1", []byte("a"), agentDir)
	writeToAgentDir("host-b", "codex", "s1", []byte("b"), agentDir)

	a, _ := os.ReadFile(filepath.Join(agentDir, "_synced", "host-a", "s1.jsonl"))
	b, _ := os.ReadFile(filepath.Join(agentDir, "_synced", "host-b", "s1.jsonl"))
	if string(a) != "a" || string(b) != "b" {
		t.Error("host isolation failed")
	}
}

// mockTransport is an in-memory transport for testing sync orchestration.
type mockTransport struct {
	files      map[string][]byte
	folders    map[string]bool
	maxChunk   int64
	verifyErr  error
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		files:   make(map[string][]byte),
		folders: make(map[string]bool),
	}
}

func (m *mockTransport) CreateFolder(path string) error {
	m.folders[path] = true
	return nil
}

func (m *mockTransport) ListFiles(path string) ([]transport.FileInfo, error) {
	prefix := path
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	seen := make(map[string]bool)
	var result []transport.FileInfo
	for p := range m.folders {
		if p == path {
			continue
		}
		if strings.HasPrefix(p, prefix) {
			rest := p[len(prefix):]
			if !strings.Contains(rest, "/") {
				if !seen[p] {
					seen[p] = true
					result = append(result, transport.FileInfo{Name: p, IsDir: true})
				}
			}
		}
	}
	for p := range m.files {
		if strings.HasPrefix(p, prefix) {
			rest := p[len(prefix):]
			if !strings.Contains(rest, "/") {
				if !seen[p] {
					seen[p] = true
					result = append(result, transport.FileInfo{Name: rest, Size: int64(len(m.files[p]))})
				}
			}
		}
	}
	return result, nil
}

func (m *mockTransport) UploadFile(folderPath, fileName string, data []byte) error {
	m.files[folderPath+"/"+fileName] = data
	return nil
}

func (m *mockTransport) DownloadFile(path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("not found: %s", path)
}

func (m *mockTransport) DeleteFile(path string) error {
	for p := range m.files {
		if strings.HasPrefix(p, path) {
			delete(m.files, p)
		}
	}
	return nil
}

func (m *mockTransport) Name() string              { return "mock" }
func (m *mockTransport) MaxChunkSize() int64       { return m.maxChunk }
func (m *mockTransport) Verify() error             { return m.verifyErr }

func TestUploadChanged_Baseline(t *testing.T) {
	tmpDir := t.TempDir()
	relPath := "sessions/claude/sess-1.jsonl"
	absPath := filepath.Join(tmpDir, relPath)
	os.MkdirAll(filepath.Dir(absPath), 0755)
	os.WriteFile(absPath, []byte(`{"messages":[{"role":"user","content":"hello"}]}`+"\n"), 0644)

	hostname, _ := os.Hostname()
	fsys := os.DirFS(tmpDir)

	cfg := &config.Config{}
	cfg.Compression.Level = 3

	st, _ := state.LoadFrom(filepath.Join(tmpDir, "state.json"))

	tr := newMockTransport()
	files := []SyncFile{
		{Path: relPath, Agent: "claude", Size: 100, Mtime: 12345},
	}

	stats, err := UploadChanged(tr, cfg, st, files, fsys, nil)
	if err != nil {
		t.Fatalf("UploadChanged: %v", err)
	}
	if stats.Synced == 0 {
		t.Error("expected at least one synced file")
	}

	// Verify baseline bundle was uploaded.
	if _, ok := tr.files[hostname+"/baseline/"+bundle.BundleFileName]; !ok {
		t.Error("baseline bundle not uploaded")
	}
}

func TestUploadChanged_Incremental(t *testing.T) {
	tmpDir := t.TempDir()
	relPath := "sessions/claude/sess-2.jsonl"
	absPath := filepath.Join(tmpDir, relPath)
	os.MkdirAll(filepath.Dir(absPath), 0755)
	os.WriteFile(absPath, []byte(`{"messages":[{"role":"user","content":"hi"}]}`+"\n"), 0644)

	hostname, _ := os.Hostname()
	fsys := os.DirFS(tmpDir)

	cfg := &config.Config{}
	cfg.Compression.Level = 3

	st, _ := state.LoadFrom(filepath.Join(tmpDir, "state.json"))
	// Pre-populate state with a different file so incremental path is taken.
	st.MarkUploaded(hostname+"/claude/sess-old", 100, 1, "", time.Now().UTC())

	tr := newMockTransport()
	tr.folders[hostname+"/incremental"] = true
	files := []SyncFile{
		{Path: relPath, Agent: "claude", Size: 100, Mtime: 99999},
	}

	stats, err := UploadChanged(tr, cfg, st, files, fsys, nil)
	if err != nil {
		t.Fatalf("UploadChanged: %v", err)
	}
	if stats.Synced == 0 {
		t.Error("expected at least one synced file")
	}

	// Verify incremental file was uploaded.
	if _, ok := tr.files[hostname+"/incremental/claude/sess-2.jsonl.zst"]; !ok {
		t.Error("incremental file not uploaded")
	}
}

func TestUploadChanged_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	relPath := "sessions/claude/sess-3.jsonl"
	absPath := filepath.Join(tmpDir, relPath)
	os.MkdirAll(filepath.Dir(absPath), 0755)
	os.WriteFile(absPath, []byte(`{"messages":[{"role":"user","content":"yo"}]}`+"\n"), 0644)

	hostname, _ := os.Hostname()
	fsys := os.DirFS(tmpDir)

	cfg := &config.Config{}
	cfg.Compression.Level = 3

	st, _ := state.LoadFrom(filepath.Join(tmpDir, "state.json"))
	// Mark the exact file as already uploaded.
	st.MarkUploaded(hostname+"/claude/sess-3", 100, 99999, "", time.Now().UTC())

	tr := newMockTransport()
	tr.folders[hostname+"/incremental"] = true
	files := []SyncFile{
		{Path: relPath, Agent: "claude", Size: 100, Mtime: 99999},
	}

	stats, err := UploadChanged(tr, cfg, st, files, fsys, nil)
	if err != nil {
		t.Fatalf("UploadChanged: %v", err)
	}
	if stats.Skipped == 0 {
		t.Error("expected file to be skipped (unchanged)")
	}
}

func TestDownloadSession(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "claude", "projects")

	raw := []byte(`{"messages":[{"role":"user","content":"downloaded"}]}`+"\n")
	compressed, err := compress.Compress(raw, 3)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}

	tr := newMockTransport()
	remotePath := "host-a/incremental/claude/sess-abc.jsonl.zst"
	tr.files[remotePath] = compressed

	session := RemoteSession{
		Key:       "host-a/claude/sess-abc",
		Computer:  "host-a",
		Agent:     "claude",
		SessionID: "sess-abc",
		Path:      remotePath,
	}

	findAgentDir := func(agent string) string {
		return agentDir
	}

	err = DownloadSession(tr, session, findAgentDir)
	if err != nil {
		t.Fatalf("DownloadSession: %v", err)
	}

	expectedPath := filepath.Join(agentDir, "_synced", "host-a", "sess-abc.jsonl")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !strings.Contains(string(data), "downloaded") {
		t.Errorf("unexpected content: %s", string(data))
	}
}
