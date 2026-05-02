package manifest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yanen-bohoon/session-conflux/internal/feishu"
)

const ManifestFileName = "manifest.json"

// SessionEntry describes one session in the manifest.
type SessionEntry struct {
	Title        string `json:"title"`
	MessageCount int    `json:"message_count"`
	FileToken    string `json:"file_token"`
	LastUpdated  string `json:"last_updated"` // ISO 8601
}

// Manifest is the root index file stored on Feishu Drive.
type Manifest struct {
	Version  int                     `json:"version"`
	Sessions map[string]SessionEntry `json:"sessions"` // key: "computer/agent/session_id"
}

// New creates an empty manifest.
func New() *Manifest {
	return &Manifest{
		Version:  1,
		Sessions: make(map[string]SessionEntry),
	}
}

// Download fetches the manifest from Feishu Drive.
// Returns nil if the manifest file doesn't exist yet.
func Download(token, folderToken string) (*Manifest, error) {
	// First find the manifest file in the folder
	fileToken, err := findManifestFile(token, folderToken)
	if err != nil {
		return nil, err
	}
	if fileToken == "" {
		return nil, nil // manifest doesn't exist yet
	}

	data, err := feishu.DownloadFile(token, fileToken)
	if err != nil {
		return nil, fmt.Errorf("download manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Sessions == nil {
		m.Sessions = make(map[string]SessionEntry)
	}
	return &m, nil
}

// Upload serializes and uploads the manifest, creating or overwriting.
// fileToken is the existing manifest file token (empty if creating).
// Returns the file token (new or existing).
func Upload(token, folderToken, existingFileToken string, m *Manifest) (string, error) {
	if m.Sessions == nil {
		m.Sessions = make(map[string]SessionEntry)
	}
	m.Version = 1
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}

	return feishu.UploadFile(token, folderToken, ManifestFileName, data)
}

// UpsertSession adds or updates a session entry and uploads the manifest.
func UpsertSession(token, folderToken, existingFileToken string, key string, entry SessionEntry) (string, error) {
	m, err := Download(token, folderToken)
	if err != nil {
		return "", err
	}
	if m == nil {
		m = New()
	}

	entry.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	m.Sessions[key] = entry

	return Upload(token, folderToken, existingFileToken, m)
}

// findManifestFile searches the folder for the manifest file.
func findManifestFile(token, folderToken string) (string, error) {
	files, err := feishu.ListFiles(token, folderToken)
	if err != nil {
		return "", fmt.Errorf("list files: %w", err)
	}
	for _, f := range files {
		if f.Name == ManifestFileName {
			return f.Token, nil
		}
	}
	return "", nil
}
