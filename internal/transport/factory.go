package transport

import (
	"fmt"

	"github.com/yanen-bohoon/session-conflux/internal/config"
)

// New creates a Transport based on cfg.Transport.Backend.
func New(cfg *config.Config) (Transport, error) {
	switch cfg.Transport.Backend {
	case "feishu":
		fc := cfg.Transport.Feishu
		return NewFeishuTransport(fc.AppID, fc.AppSecret, fc.FolderToken), nil
	case "ssh":
		return NewSSHTransport(cfg.Transport.SSH)
	case "":
		return nil, fmt.Errorf("no transport backend configured; run 'session-conflux setup'")
	default:
		return nil, fmt.Errorf("unknown transport backend: %q", cfg.Transport.Backend)
	}
}
