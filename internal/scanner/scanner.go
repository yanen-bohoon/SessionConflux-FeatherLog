package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yanen-bohoon/session-conflux/internal/parser"
)

// ScanResult is a discovered session ready for processing.
type ScanResult struct {
	Path         string
	Agent        string
	SessionID    string
	Title        string
	MessageCount int
	Size         int64
}

// Scan walks all enabled agent directories and returns discovered sessions.
// exclude is a set of agent types to skip.
func Scan(exclude map[string]bool) ([]ScanResult, error) {
	var results []ScanResult

	for _, def := range parser.AllAgents {
		if exclude[def.Type] {
			continue
		}
		dirs := parser.ResolveAgentDirs(def)
		for _, dir := range dirs {
			found, err := discoverInDir(dir, def.Type)
			if err != nil {
				// Skip inaccessible directories
				continue
			}
			results = append(results, found...)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Agent != results[j].Agent {
			return results[i].Agent < results[j].Agent
		}
		return results[i].SessionID < results[j].SessionID
	})
	return results, nil
}

// discoverInDir finds all JSONL files under a directory.
func discoverInDir(root, agent string) ([]ScanResult, error) {
	var results []ScanResult
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		// Extract metadata quickly
		meta, err := parser.ExtractMeta(path)
		if err != nil || meta.MessageCount == 0 {
			return nil
		}

		results = append(results, ScanResult{
			Path:         path,
			Agent:        agent,
			SessionID:    meta.SessionID,
			Title:        meta.Title,
			MessageCount: meta.MessageCount,
			Size:         info.Size(),
		})
		return nil
	})
	if err != nil && len(results) == 0 {
		return nil, fmt.Errorf("walk: %w", err)
	}
	return results, nil
}
