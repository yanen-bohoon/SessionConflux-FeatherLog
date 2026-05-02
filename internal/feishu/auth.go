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

type tokenCache struct {
    mu        sync.Mutex
    token     string
    expiresAt time.Time
}

var cache = &tokenCache{}

// GetTenantToken returns a cached tenant access token, refreshing if needed.
func GetTenantToken(appID, appSecret string) (string, error) {
    cache.mu.Lock()
    defer cache.mu.Unlock()

    if cache.token != "" && time.Now().Before(cache.expiresAt) {
        return cache.token, nil
    }

    body := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, appID, appSecret)
    resp, err := http.Post(
        BaseURL+"/auth/v3/tenant_access_token/internal",
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

    cache.token = result.TenantAccessToken
    cache.expiresAt = time.Now().Add(time.Duration(result.Expire)*time.Second - 5*time.Minute)
    if result.Expire == 0 {
        cache.expiresAt = time.Now().Add(2*time.Hour - 5*time.Minute)
    }
    return cache.token, nil
}
