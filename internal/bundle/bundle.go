package bundle

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

const BundleFileName = "bundle.tar.zst"

// Pack creates a tar.zst archive containing all session files.
// Each entry is named: hostname/agent/session_id.jsonl
func Pack(sessions map[string][]byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.EncoderLevel(level)))
	if err != nil {
		return nil, fmt.Errorf("zstd encoder: %w", err)
	}
	tw := tar.NewWriter(enc)

	for name, data := range sessions {
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(data)),
			Mode: 0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("tar write header %s: %w", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			return nil, fmt.Errorf("tar write %s: %w", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("zstd close: %w", err)
	}
	return buf.Bytes(), nil
}

// Unpack extracts a tar.zst archive and returns all files keyed by their
// tar entry name (hostname/agent/session_id.jsonl).
func Unpack(data []byte) (map[string][]byte, error) {
	dec, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("zstd reader: %w", err)
	}
	defer dec.Close()

	files := make(map[string][]byte)
	tr := tar.NewReader(dec)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar next: %w", err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("tar read %s: %w", hdr.Name, err)
		}
		files[hdr.Name] = content
	}
	return files, nil
}

// WriteToAgentDir writes session data to the appropriate agent directory
// so that AgentsView's fsnotify discovers it.
func WriteToAgentDir(agent, sessionID string, data []byte, agentDir string) error {
	targetDir := filepath.Join(agentDir, "_synced")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	targetFile := filepath.Join(targetDir, sessionID+".jsonl")
	if err := os.WriteFile(targetFile, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
