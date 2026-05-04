package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yanen-bohoon/session-conflux/pkg/feishu"
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
