package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// FeishuConfig holds Feishu/Lark API credentials and target folder.
type FeishuConfig struct {
	AppID       string `toml:"app_id"`
	AppSecret   string `toml:"app_secret"`
	FolderToken string `toml:"folder_token"` // empty = auto-create "SessionConflux" folder
}

// SyncConfig controls sync schedule and direction.
type SyncConfig struct {
	Schedule  string `toml:"schedule"`  // "02:00" default
	Direction string `toml:"direction"` // "both" | "upload" | "download"
}

// AgentsConfig controls which agents to skip during sync.
type AgentsConfig struct {
	Exclude []string `toml:"exclude"` // agent names to skip
}

// CompressionConfig controls zstd compression level.
type CompressionConfig struct {
	Level int `toml:"level"` // zstd 1-22, default 3
}

// Config is the top-level configuration structure.
type Config struct {
	Feishu      FeishuConfig      `toml:"feishu"`
	Sync        SyncConfig        `toml:"sync"`
	Agents      AgentsConfig      `toml:"agents"`
	Compression CompressionConfig `toml:"compression"`
}

// DefaultPath returns the default config file path (~/.session-conflux/config.toml).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".session-conflux", "config.toml"), nil
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Sync: SyncConfig{
			Schedule:  "02:00",
			Direction: "both",
		},
		Compression: CompressionConfig{
			Level: 3,
		},
	}
}

// Load reads config from the default path.
// If the file doesn't exist, returns a Config with defaults.
func Load() (*Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads config from a specific path.
func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return cfg, nil
}

// Save writes config to the default path.
func Save(cfg *Config) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return cfg.SaveTo(path)
}

// SaveTo writes config to a specific path.
func (c *Config) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}
