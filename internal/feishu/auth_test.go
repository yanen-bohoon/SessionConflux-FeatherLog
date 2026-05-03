package feishu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("app-1", "secret-1")
	if c.appID != "app-1" {
		t.Errorf("appID = %q, want %q", c.appID, "app-1")
	}
	if c.appSecret != "secret-1" {
		t.Errorf("appSecret = %q, want %q", c.appSecret, "secret-1")
	}
	if c.httpClient != http.DefaultClient {
		t.Error("httpClient should default to http.DefaultClient")
	}
	if c.baseURL != BaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, BaseURL)
	}
}

func TestSetHTTPClient(t *testing.T) {
	c := NewClient("app-1", "secret-1")
	custom := &http.Client{}
	c.SetHTTPClient(custom)
	if c.httpClient != custom {
		t.Error("SetHTTPClient did not replace httpClient")
	}
}

func TestSetBaseURL(t *testing.T) {
	c := NewClient("app-1", "secret-1")
	c.SetBaseURL("http://test.local")
	if c.baseURL != "http://test.local" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://test.local")
	}
}

func TestGetTenantToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v3/tenant_access_token/internal" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body struct {
			AppID     string `json:"app_id"`
			AppSecret string `json:"app_secret"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.AppID != "test-app" {
			t.Errorf("app_id = %q, want %q", body.AppID, "test-app")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"code":                0,
			"msg":                 "ok",
			"tenant_access_token": "token-abc123",
			"expire":              7200,
		})
	}))
	defer srv.Close()

	c := NewClient("test-app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	tok, err := c.GetTenantToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "token-abc123" {
		t.Errorf("token = %q, want %q", tok, "token-abc123")
	}
}

func TestGetTenantToken_Cache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{
			"code":                0,
			"msg":                 "ok",
			"tenant_access_token": "cached-token",
			"expire":              7200,
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	// First call — hits server.
	tok1, err := c.GetTenantToken()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if tok1 != "cached-token" {
		t.Errorf("got %q, want %q", tok1, "cached-token")
	}
	if calls != 1 {
		t.Errorf("calls after first fetch = %d, want 1", calls)
	}

	// Second call — must use cache.
	tok2, err := c.GetTenantToken()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if tok2 != "cached-token" {
		t.Errorf("got %q, want %q", tok2, "cached-token")
	}
	if calls != 1 {
		t.Errorf("server was hit %d times, want 1 (cache not used)", calls)
	}
}

func TestGetTenantToken_ExpiredRefresh(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{
			"code":                0,
			"msg":                 "ok",
			"tenant_access_token": "token-after-refresh",
			"expire":              0, // triggers default 2h expiry
		})
	}))
	defer srv.Close()

	c := NewClient("app", "secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	// Seed an expired token.
	c.token = "stale-token"
	c.expiresAt = time.Now().Add(-1 * time.Minute)

	tok, err := c.GetTenantToken()
	if err != nil {
		t.Fatalf("refresh call: %v", err)
	}
	if tok != "token-after-refresh" {
		t.Errorf("got %q, want %q", tok, "token-after-refresh")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestGetTenantToken_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 999,
			"msg":  "invalid app secret",
		})
	}))
	defer srv.Close()

	c := NewClient("app", "bad-secret")
	c.SetBaseURL(srv.URL)
	c.httpClient = srv.Client()

	_, err := c.GetTenantToken()
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}
