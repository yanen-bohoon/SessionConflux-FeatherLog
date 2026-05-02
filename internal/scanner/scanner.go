package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yanen-bohoon/session-conflux/internal/parser"
)

// ScanResult is a discovered session ready for display.
type ScanResult struct {
	Path      string
	Agent     string
	SessionID string
	Size      int64
}

// Scan walks all enabled agent directories and returns discovered sessions.
// Only stats files — no file content is read.
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

// discoverInDir finds all JSONL files under a directory using stat only.
func discoverInDir(root, agent string) ([]ScanResult, error) {
	var results []ScanResult
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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

		name := info.Name()
		sessionID := name[:len(name)-6] // trim ".jsonl"

		results = append(results, ScanResult{
			Path:      path,
			Agent:     agent,
			SessionID: sessionID,
			Size:      info.Size(),
		})
		return nil
	})
	if err != nil && len(results) == 0 {
		return nil, err
	}
	return results, nil
}
