package feishu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const BaseURL = "https://open.feishu.cn/open-apis"

// Client is a Feishu API client that caches tenant access tokens internally.
// The zero value is unusable; use NewClient.
type Client struct {
	httpClient *http.Client
	appID      string
	appSecret  string
	baseURL    string

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// NewClient creates a client for the given Feishu app credentials.
func NewClient(appID, appSecret string) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		appID:      appID,
		appSecret:  appSecret,
		baseURL:    BaseURL,
	}
}

// SetBaseURL overrides the API base URL (for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// SetHTTPClient replaces the underlying HTTP client. Useful for tests.
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.httpClient = hc
}

// GetTenantToken returns a cached tenant access token, refreshing if needed.
func (c *Client) GetTenantToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.expiresAt) {
		return c.token, nil
	}

	body := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, c.appID, c.appSecret)
	resp, err := c.httpClient.Post(
		c.baseURL+"/auth/v3/tenant_access_token/internal",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("auth decode failed: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("auth API error: code=%d msg=%s", result.Code, result.Msg)
	}

	c.token = result.TenantAccessToken
	c.expiresAt = time.Now().Add(time.Duration(result.Expire)*time.Second - 5*time.Minute)
	if result.Expire == 0 {
		c.expiresAt = time.Now().Add(2*time.Hour - 5*time.Minute)
	}
	return c.token, nil
}
