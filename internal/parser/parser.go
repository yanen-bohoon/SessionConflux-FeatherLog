package parser

import "time"

// DiscoveredFile represents a session file found on disk.
type DiscoveredFile struct {
	Path  string
	Agent string
	Mtime time.Time
	Size  int64
}

// SessionMeta holds minimal metadata extracted from a session file.
type SessionMeta struct {
	SessionID    string
	Title        string
	MessageCount int
}
