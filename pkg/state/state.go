package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry tracks upload and download state for a single session.
// Key format: "computer/agent/session_id"
type Entry struct {
	FileSize        int64  `json:"file_size"`
	Mtime           int64  `json:"mtime"`
	FileToken       string `json:"file_token"`
	LastUploaded    time.Time `json:"last_uploaded"`
	DownloadedToken string    `json:"downloaded_token,omitempty"`
	LastDownloaded  time.Time `json:"last_downloaded,omitempty"`
}

// Store manages the state.json file.
type Store struct {
	path    string
	entries map[string]Entry // key -> entry
}

// DefaultPath returns the default state file path (~/.session-conflux/state.json).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".session-conflux", "state.json"), nil
}

// Load reads state from the default path.
// Returns an empty store if the file doesn't exist.
func Load() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads state from a specific path.
func LoadFrom(path string) (*Store, error) {
	s := &Store{
		path:    path,
		entries: make(map[string]Entry),
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	if err := json.Unmarshal(data, &s.entries); err != nil {
		// corrupted file, start fresh
		s.entries = make(map[string]Entry)
	}
	return s, nil
}

// Get returns the entry for a key, and whether it exists.
func (s *Store) Get(key string) (Entry, bool) {
	e, ok := s.entries[key]
	return e, ok
}

// HasChanged returns true if the session file size or mtime differs from state (or is new).
func (s *Store) HasChanged(key string, fileSize, mtime int64) bool {
	e, ok := s.entries[key]
	if !ok {
		return fileSize > 0
	}
	return fileSize != e.FileSize || mtime != e.Mtime
}

// MarkUploaded records a successful upload.
func (s *Store) MarkUploaded(key string, fileSize, mtime int64, fileToken string, t time.Time) {
	e := s.entries[key]
	e.FileSize = fileSize
	e.Mtime = mtime
	e.FileToken = fileToken
	e.LastUploaded = t
	s.entries[key] = e
}

// NeedsDownload returns true if the remote file token differs from what was
// last downloaded (or if never downloaded).
func (s *Store) NeedsDownload(key string, fileToken string) bool {
	e, ok := s.entries[key]
	if !ok {
		return true
	}
	return e.DownloadedToken != fileToken
}

// MarkDownloaded records a successful download.
func (s *Store) MarkDownloaded(key string, fileToken string, t time.Time) {
	e := s.entries[key]
	e.DownloadedToken = fileToken
	e.LastDownloaded = t
	s.entries[key] = e
}

// All returns all entries.
func (s *Store) All() map[string]Entry {
	return s.entries
}

// RemoveAll deletes all entries whose key starts with prefix (typically
// a hostname followed by "/").
func (s *Store) RemoveAll(prefix string) {
	for k := range s.entries {
		if len(k) > len(prefix) && k[:len(prefix)+1] == prefix+"/" {
			delete(s.entries, k)
		}
	}
}

// Save writes the state to disk.
func (s *Store) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp state: %w", err)
	}
	return os.Rename(tmp, s.path)
}
