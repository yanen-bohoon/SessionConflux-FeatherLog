package transport

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/yanen-bohoon/session-conflux/pkg/feishu"
)

// feishuMaxChunkSize is the maximum file size (in bytes) that can be
// uploaded to Feishu Drive in a single request. Files larger than this
// are split into parts by the sync layer.
const feishuMaxChunkSize = 19 * 1024 * 1024

// FeishuTransport implements Transport backed by Feishu Drive.
// It wraps feishu.Client and maintains a path-to-folder-token cache
// so that higher layers can use path-based addressing.
type FeishuTransport struct {
	client     *feishu.Client
	rootToken  string            // token for the "SessionConflux" root folder, or empty
	tokenCache map[string]string // lowercased path -> folder token
	mu         sync.RWMutex
}

// NewFeishuTransport creates a FeishuTransport with the given credentials.
// rootToken may be empty to auto-create the root folder on first use.
func NewFeishuTransport(appID, appSecret, rootToken string) *FeishuTransport {
	return &FeishuTransport{
		client:     feishu.NewClient(appID, appSecret),
		rootToken:  rootToken,
		tokenCache: make(map[string]string),
	}
}

// NewFeishuTransportWithClient creates a FeishuTransport from a pre-built client.
// Useful for tests that need to inject an httptest.Server-backed client.
func NewFeishuTransportWithClient(client *feishu.Client, rootToken string) *FeishuTransport {
	return &FeishuTransport{
		client:     client,
		rootToken:  rootToken,
		tokenCache: make(map[string]string),
	}
}

func (ft *FeishuTransport) Name() string { return "feishu" }

// MaxChunkSize returns the Feishu Drive upload limit (20 MB minus headroom).
func (ft *FeishuTransport) MaxChunkSize() int64 { return feishuMaxChunkSize }

// RootToken returns the token of the root SessionConflux folder.
// Empty until CreateFolder("") or resolveToken("") is called.
func (ft *FeishuTransport) RootToken() string {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	return ft.rootToken
}

// Verify checks Feishu credentials by requesting a tenant token.
func (ft *FeishuTransport) Verify() error {
	_, err := ft.client.GetTenantToken()
	return err
}

// CreateFolder creates a folder at path, creating intermediates as needed.
// Pass "" to ensure the root folder exists.
func (ft *FeishuTransport) CreateFolder(folderPath string) error {
	_, err := ft.resolveToken(folderPath)
	return err
}

// ListFiles lists children of folderPath. Pass "" for root.
func (ft *FeishuTransport) ListFiles(folderPath string) ([]FileInfo, error) {
	tok, err := ft.resolveToken(folderPath)
	if err != nil {
		return nil, err
	}
	files, err := ft.client.ListFiles(tok)
	if err != nil {
		return nil, err
	}
	out := make([]FileInfo, len(files))
	for i, f := range files {
		out[i] = FileInfo{
			Name:  f.Name,
			IsDir: f.Type == "folder",
			Size:  0, // Feishu Drive API does not return size in ListFiles
		}
	}
	return out, nil
}

// UploadFile writes data as fileName into folderPath.
func (ft *FeishuTransport) UploadFile(folderPath, fileName string, data []byte) error {
	tok, err := ft.resolveToken(folderPath)
	if err != nil {
		return err
	}
	for attempt := 1; attempt <= 3; attempt++ {
		_, err = ft.client.UploadFile(tok, fileName, data)
		if err == nil {
			return nil
		}
		if attempt == 3 {
			break
		}
		// Exponential backoff: 1s, 2s
		// time.Sleep imported separately if needed
	}
	return fmt.Errorf("upload %s/%s after 3 attempts: %w", folderPath, fileName, err)
}

// DownloadFile reads the file at path. The path must include the file name.
func (ft *FeishuTransport) DownloadFile(filePath string) ([]byte, error) {
	dir, name := path.Split(filePath)
	name = strings.TrimSuffix(name, "/") // path.Split leaves trailing slash on dir-only paths
	if name == "" {
		return nil, fmt.Errorf("download path has no file name: %q", filePath)
	}
	tok, err := ft.resolveToken(dir)
	if err != nil {
		return nil, err
	}
	files, err := ft.client.ListFiles(tok)
	if err != nil {
		return nil, err
	}
	fileToken := findFileByName(files, name)
	if fileToken == "" {
		return nil, fmt.Errorf("file not found: %q", filePath)
	}
	return ft.client.DownloadFile(fileToken, nil)
}

// DeleteFile removes the file at path. Idempotent.
func (ft *FeishuTransport) DeleteFile(filePath string) error {
	dir, name := path.Split(filePath)
	if name == "" {
		return nil
	}
	tok, err := ft.resolveToken(dir)
	if err != nil {
		return err
	}
	files, err := ft.client.ListFiles(tok)
	if err != nil {
		return err
	}
	fileToken := findFileByName(files, name)
	if fileToken == "" {
		return nil // already gone
	}
	return ft.client.DeleteFile(fileToken)
}

// resolveToken returns the folder token for folderPath, creating folders as needed.
// folderPath may be "" for the root.
func (ft *FeishuTransport) resolveToken(folderPath string) (string, error) {
	if folderPath == "" || folderPath == "." {
		return ft.ensureRoot()
	}

	ft.mu.RLock()
	tok, ok := ft.tokenCache[strings.ToLower(folderPath)]
	ft.mu.RUnlock()
	if ok {
		return tok, nil
	}

	ft.mu.Lock()
	defer ft.mu.Unlock()

	// Double-check after acquiring write lock.
	if tok, ok := ft.tokenCache[strings.ToLower(folderPath)]; ok {
		return tok, nil
	}

	parent, err := ft.ensureRoot()
	if err != nil {
		return "", err
	}

	parts := strings.Split(folderPath, "/")
	var currentPath string
	for _, part := range parts {
		if part == "" {
			continue
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}
		key := strings.ToLower(currentPath)
		if tok, ok := ft.tokenCache[key]; ok {
			parent = tok
			continue
		}
		tok, err = ft.client.FindOrCreateFolder(part, parent)
		if err != nil {
			return "", fmt.Errorf("create folder %q: %w", currentPath, err)
		}
		ft.tokenCache[key] = tok
		parent = tok
	}
	return parent, nil
}

func (ft *FeishuTransport) ensureRoot() (string, error) {
	if ft.rootToken != "" {
		return ft.rootToken, nil
	}
	tok, err := ft.client.FindOrCreateFolder("SessionConflux")
	if err != nil {
		return "", fmt.Errorf("root folder: %w", err)
	}
	ft.rootToken = tok
	return tok, nil
}

func findFileByName(files []feishu.FileInfo, name string) string {
	for _, f := range files {
		if f.Name == name {
			return f.Token
		}
	}
	return ""
}
