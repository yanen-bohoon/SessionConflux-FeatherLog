package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yanen-bohoon/session-conflux/pkg/feishu"
)

// newTestFeishuTransport creates a FeishuTransport pointed at a test server.
func newTestFeishuTransport(srv *httptest.Server, rootToken string) *FeishuTransport {
	c := feishu.NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.SetHTTPClient(srv.Client())
	return &FeishuTransport{
		client:     c,
		rootToken:  rootToken,
		tokenCache: make(map[string]string),
	}
}

// Standard Feishu auth response used by every test handler.
func authResp() map[string]any {
	return map[string]any{
		"code":                0,
		"tenant_access_token": "test-token",
		"expire":              7200,
	}
}

func TestFeishuTransport_Name(t *testing.T) {
	ft := &FeishuTransport{client: feishu.NewClient("a", "s")}
	if ft.Name() != "feishu" {
		t.Errorf("Name = %q, want feishu", ft.Name())
	}
}

func TestFeishuTransport_CreateFolder_Root(t *testing.T) {
	var createCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		if strings.Contains(r.URL.Path, "create_folder") {
			createCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"token": "root-folder-token"},
			})
			return
		}
		// ListFiles returns empty — forces create
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"files": []any{}, "has_more": false},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "")
	err := ft.CreateFolder("")
	if err != nil {
		t.Fatalf("CreateFolder empty: %v", err)
	}
	if !createCalled {
		t.Error("should have created root folder 'SessionConflux'")
	}
}

func TestFeishuTransport_CreateFolder_Nested(t *testing.T) {
	createNames := make(map[string]bool)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		if strings.Contains(r.URL.Path, "create_folder") {
			// Parse the request body to get the folder name
			body := make([]byte, 256)
			n, _ := r.Body.Read(body)
			bstr := string(body[:n])
			folderName := "unknown"
			for _, name := range []string{"SessionConflux", "mac-studio", "baseline"} {
				if strings.Contains(bstr, name) {
					createNames[name] = true
					folderName = name
				}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"token": "token-" + folderName},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"files": []any{}, "has_more": false},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "")
	err := ft.CreateFolder("mac-studio/baseline")
	if err != nil {
		t.Fatalf("CreateFolder nested: %v", err)
	}
	if !createNames["SessionConflux"] {
		t.Error("should have created root folder 'SessionConflux'")
	}
	if !createNames["mac-studio"] {
		t.Error("should have created 'mac-studio' folder")
	}
	if !createNames["baseline"] {
		t.Error("should have created 'baseline' folder")
	}
}

func TestFeishuTransport_CreateFolder_Idempotent(t *testing.T) {
	var createCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		if strings.Contains(r.URL.Path, "create_folder") {
			createCalls++
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"token": "folder-token"},
			})
			return
		}
		// Second ListFiles returns the existing folder
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files": []map[string]any{
					{"token": "existing-token", "name": "host1", "type": "folder"},
				},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "root-tok")
	// First call: create root (uses auto-root logic)
	if err := ft.CreateFolder("host1"); err != nil {
		t.Fatalf("first CreateFolder: %v", err)
	}
	firstCalls := createCalls

	// Second call: should find existing, no new create
	if err := ft.CreateFolder("host1"); err != nil {
		t.Fatalf("second CreateFolder: %v", err)
	}
	if createCalls != firstCalls {
		t.Errorf("second call triggered create (calls %d -> %d)", firstCalls, createCalls)
	}
}

func TestFeishuTransport_ListFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files": []map[string]any{
					{"token": "ft1", "name": "bundle.tar.zst", "type": "file"},
					{"token": "ft2", "name": "incremental", "type": "folder"},
				},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "root-tok")
	files, err := ft.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].Name != "bundle.tar.zst" || files[0].IsDir {
		t.Errorf("file[0] = %+v, want {bundle.tar.zst false}", files[0])
	}
	if files[1].Name != "incremental" || !files[1].IsDir {
		t.Errorf("file[1] = %+v, want {incremental true}", files[1])
	}
}

func TestFeishuTransport_UploadFile(t *testing.T) {
	var uploadedName string
	var uploadedData []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		if strings.Contains(r.URL.Path, "upload_all") {
			// Parse multipart form to capture the upload
			r.ParseMultipartForm(10 << 20)
			uploadedName = r.FormValue("file_name")
			f, _, _ := r.FormFile("file")
			if f != nil {
				uploadedData = make([]byte, 1024)
				n, _ := f.Read(uploadedData)
				uploadedData = uploadedData[:n]
				f.Close()
			}
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"file_token": "uploaded-file-token"},
			})
			return
		}
		// ListFiles for root
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files": []map[string]any{
					{"token": "host-tok", "name": "mac-studio", "type": "folder"},
				},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "root-tok")
	err := ft.UploadFile("mac-studio/incremental", "claude/sess-123.jsonl.zst", []byte("hello"))
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if uploadedName != "claude/sess-123.jsonl.zst" {
		t.Errorf("uploaded name = %q", uploadedName)
	}
	if string(uploadedData) != "hello" {
		t.Errorf("uploaded data = %q", string(uploadedData))
	}
}

func TestFeishuTransport_DownloadFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		if strings.Contains(r.URL.Path, "/download") {
			w.Header().Set("Content-Length", "5")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("world"))
			return
		}
		// ListFiles returns the target file
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files": []map[string]any{
					{"token": "dl-file-token", "name": "sess-123.jsonl.zst", "type": "file"},
				},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "root-tok")
	data, err := ft.DownloadFile("mac-studio/incremental/sess-123.jsonl.zst")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("data = %q, want world", string(data))
	}
}

func TestFeishuTransport_DownloadFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		// ListFiles returns empty — file not found
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"files": []any{}, "has_more": false},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "root-tok")
	_, err := ft.DownloadFile("nonexistent/file.jsonl.zst")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFeishuTransport_DeleteFile(t *testing.T) {
	var deletedFileToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		if r.Method == "DELETE" {
			// Extract file token from URL
			deletedFileToken = strings.TrimPrefix(r.URL.Path, "/drive/v1/files/")
			deletedFileToken = strings.TrimSuffix(deletedFileToken, "?type=file")
			w.WriteHeader(http.StatusOK)
			return
		}
		// ListFiles returns the target file
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files": []map[string]any{
					{"token": "del-file-token", "name": "old-session.jsonl.zst", "type": "file"},
				},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "root-tok")
	err := ft.DeleteFile("mac-studio/incremental/old-session.jsonl.zst")
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if deletedFileToken != "del-file-token" {
		t.Errorf("deleted token = %q, want del-file-token", deletedFileToken)
	}
}

func TestFeishuTransport_DeleteFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		// ListFiles returns empty — file already gone
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"files": []any{}, "has_more": false},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "root-tok")
	err := ft.DeleteFile("already-gone.jsonl.zst")
	if err != nil {
		t.Fatalf("DeleteFile should be idempotent, got error: %v", err)
	}
}

func TestFeishuTransport_TokenCache(t *testing.T) {
	var listCalls int
	var createCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(authResp())
			return
		}
		if strings.Contains(r.URL.Path, "create_folder") {
			createCalls++
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{"token": "folder-tok"},
			})
			return
		}
		listCalls++
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"files": []any{}, "has_more": false},
		})
	}))
	defer srv.Close()

	ft := newTestFeishuTransport(srv, "")
	// First call — creates root + host1
	ft.CreateFolder("host1")
	create1 := createCalls
	// Second call — should hit cache, no new API calls for host1
	ft.CreateFolder("host1")
	if createCalls != create1 {
		t.Errorf("host1 create called again (calls %d -> %d)", create1, createCalls)
	}
}
