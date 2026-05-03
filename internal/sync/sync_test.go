package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
)

func newTestClient(srv *httptest.Server) *feishu.Client {
	c := feishu.NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.SetHTTPClient(srv.Client())
	return c
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

	client := newTestClient(srv)
	cfg := &config.Config{Feishu: config.FeishuConfig{FolderToken: "l1-token"}}

	sessions, err := ListRemoteSessions(client, cfg)
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

	client := newTestClient(srv)
	cfg := &config.Config{Feishu: config.FeishuConfig{FolderToken: "l1-token"}}

	sessions, err := ListRemoteSessions(client, cfg)
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

func TestListRemoteSessions_AutoFolder(t *testing.T) {
	// When FolderToken is empty, it should auto-create the root folder.
	var createCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		if strings.Contains(r.URL.Path, "create_folder") {
			createCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"token": "auto-folder-token"},
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

	client := newTestClient(srv)
	cfg := &config.Config{} // empty FolderToken

	_, err := ListRemoteSessions(client, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("should have called CreateFolder for missing root token")
	}
}

func TestFindFileInList(t *testing.T) {
	files := []feishu.FileInfo{
		{Token: "t1", Name: "a.jsonl.zst"},
		{Token: "t2", Name: "b.jsonl.zst"},
	}
	if got := findFileInList(files, "a.jsonl.zst"); got != "t1" {
		t.Errorf("got %q, want t1", got)
	}
	if got := findFileInList(files, "c.jsonl.zst"); got != "" {
		t.Errorf("got %q, want empty", got)
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

func TestFormatBytesOrUnknown(t *testing.T) {
	if got := formatBytesOrUnknown(0); got != "?? B" {
		t.Errorf("formatBytesOrUnknown(0) = %q, want %q", got, "?? B")
	}
	if got := formatBytesOrUnknown(-1); got != "?? B" {
		t.Errorf("formatBytesOrUnknown(-1) = %q, want %q", got, "?? B")
	}
	if got := formatBytesOrUnknown(500); got != "500 B" {
		t.Errorf("formatBytesOrUnknown(500) = %q, want %q", got, "?? B")
	}
}
