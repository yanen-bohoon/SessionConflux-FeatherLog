package feishu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateFolder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}

		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		var body struct {
			Name        string `json:"name"`
			FolderToken string `json:"folder_token"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "TestFolder" {
			t.Errorf("name = %q, want %q", body.Name, "TestFolder")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{"token": "folder-token-123"},
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	tok, err := c.CreateFolder("TestFolder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "folder-token-123" {
		t.Errorf("token = %q, want %q", tok, "folder-token-123")
	}
}

func TestCreateFolder_WithParent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}

		var body struct {
			Name        string `json:"name"`
			FolderToken string `json:"folder_token"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.FolderToken != "parent-456" {
			t.Errorf("folder_token = %q, want %q", body.FolderToken, "parent-456")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{"token": "child-token"},
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	tok, err := c.CreateFolder("Child", "parent-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "child-token" {
		t.Errorf("token = %q, want %q", tok, "child-token")
	}
}

func TestCreateFolder_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 1001,
			"msg":  "name already exists",
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	_, err := c.CreateFolder("dup")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUploadFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}

		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		// Parse multipart form (max 1MB).
		r.ParseMultipartForm(1 << 20)
		if r.FormValue("file_name") != "test.jsonl.zst" {
			t.Errorf("file_name = %q", r.FormValue("file_name"))
		}
		if r.FormValue("parent_type") != "explorer" {
			t.Errorf("parent_type = %q", r.FormValue("parent_type"))
		}
		if r.FormValue("parent_node") != "folder-abc" {
			t.Errorf("parent_node = %q", r.FormValue("parent_node"))
		}

		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{"file_token": "file-token-xyz"},
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	ft, err := c.UploadFile("folder-abc", "test.jsonl.zst", []byte("hello world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ft != "file-token-xyz" {
		t.Errorf("file_token = %q, want %q", ft, "file-token-xyz")
	}
}

func TestUploadFile_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 2001,
			"msg":  "file too large",
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	_, err := c.UploadFile("f", "big.zst", make([]byte, 100))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("downloaded content"))
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	data, err := c.DownloadFile("file-token-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "downloaded content" {
		t.Errorf("data = %q, want %q", string(data), "downloaded content")
	}
}

func TestDownloadFile_WithProgress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		w.Header().Set("Content-Length", "3")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("abc"))
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	var progressCalls []int64
	data, err := c.DownloadFile("ft", func(downloaded, total int64) {
		progressCalls = append(progressCalls, downloaded)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "abc" {
		t.Errorf("data = %q", string(data))
	}
	if len(progressCalls) < 1 {
		t.Error("progress callback was never called")
	}
	// First call should be 0.
	if progressCalls[0] != 0 {
		t.Errorf("first progress = %d, want 0", progressCalls[0])
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":404,"msg":"file not found"}`))
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	_, err := c.DownloadFile("missing-token", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestListFiles_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{
				"files": []map[string]any{
					{"token": "t1", "name": "file1.jsonl.zst", "type": "file"},
					{"token": "t2", "name": "folder1", "type": "folder"},
				},
				"has_more":         false,
				"next_page_token":  "",
			},
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	files, err := c.ListFiles("some-folder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].Name != "file1.jsonl.zst" || files[0].Type != "file" {
		t.Errorf("file[0] = {%q, %q}", files[0].Name, files[0].Type)
	}
	if files[1].Name != "folder1" || files[1].Type != "folder" {
		t.Errorf("file[1] = {%q, %q}", files[1].Name, files[1].Type)
	}
}

func TestListFiles_Pagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}

		page++
		if page == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"files": []map[string]any{
						{"token": "t1", "name": "a", "type": "file"},
					},
					"has_more":        true,
					"next_page_token": "page2",
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"files": []map[string]any{
						{"token": "t2", "name": "b", "type": "file"},
					},
					"has_more": false,
				},
			})
		}
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	files, err := c.ListFiles("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files across pages, want 2", len(files))
	}
}

func TestListFiles_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code": 3001,
			"msg":  "permission denied",
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	_, err := c.ListFiles("secret-folder")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindOrCreateFolder_Existing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		// ListFiles — return existing folder.
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files": []map[string]any{
					{"token": "existing-folder", "name": "MyFolder", "type": "folder"},
				},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	tok, err := c.FindOrCreateFolder("MyFolder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "existing-folder" {
		t.Errorf("token = %q, want %q", tok, "existing-folder")
	}
}

func TestFindOrCreateFolder_Create(t *testing.T) {
	createCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		if strings.Contains(r.URL.Path, "create_folder") {
			createCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{"token": "new-folder"},
			})
			return
		}
		// ListFiles — return empty.
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"files":    []map[string]any{},
				"has_more": false,
			},
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	tok, err := c.FindOrCreateFolder("NewFolder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "new-folder" {
		t.Errorf("token = %q, want %q", tok, "new-folder")
	}
	if !createCalled {
		t.Error("CreateFolder was not called")
	}
}

func TestDeleteFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	if err := c.DeleteFile("ft"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	// DeleteFile treats 404 as success (idempotent).
	if err := c.DeleteFile("already-gone"); err != nil {
		t.Fatalf("expected nil for 404, got: %v", err)
	}
}

func TestDeleteFile_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/auth/") {
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "t",
				"expire":              7200,
			})
			return
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("no permission"))
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	if err := c.DeleteFile("protected"); err == nil {
		t.Fatal("expected error for 403")
	}
}
