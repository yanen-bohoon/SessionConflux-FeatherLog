package parser

import (
	"os"
	"path/filepath"
)

type AgentDef struct {
	Type        string
	DisplayName string
	DefaultDirs []string // relative to $HOME
}

var AllAgents = []AgentDef{
	{Type: "claude", DisplayName: "Claude Code", DefaultDirs: []string{".claude/projects"}},
	{Type: "codex", DisplayName: "Codex", DefaultDirs: []string{".codex/sessions"}},
	{Type: "copilot", DisplayName: "Copilot", DefaultDirs: []string{".copilot"}},
	{Type: "gemini", DisplayName: "Gemini CLI", DefaultDirs: []string{".gemini"}},
	{Type: "opencode", DisplayName: "OpenCode", DefaultDirs: []string{".local/share/opencode"}},
	{Type: "openhands", DisplayName: "OpenHands", DefaultDirs: []string{".openhands/conversations"}},
	{Type: "cursor", DisplayName: "Cursor", DefaultDirs: []string{".cursor/projects"}},
	{Type: "amp", DisplayName: "Amp", DefaultDirs: []string{".local/share/amp/threads"}},
	{Type: "zencoder", DisplayName: "Zencoder", DefaultDirs: []string{".zencoder/sessions"}},
	{Type: "iflow", DisplayName: "iFlow", DefaultDirs: []string{".iflow/projects"}},
	{Type: "vscode-copilot", DisplayName: "VS Code Copilot", DefaultDirs: []string{
		".vscode/extensions/github.copilot-chat",
	}},
	{Type: "pi", DisplayName: "Pi", DefaultDirs: []string{".pi/agent/sessions"}},
	{Type: "openclaw", DisplayName: "OpenClaw", DefaultDirs: []string{".openclaw/agents"}},
	{Type: "kimi", DisplayName: "Kimi", DefaultDirs: []string{".kimi/sessions"}},
	{Type: "claude-ai", DisplayName: "Claude.ai", DefaultDirs: []string{}},
	{Type: "chatgpt", DisplayName: "ChatGPT", DefaultDirs: []string{}},
	{Type: "kiro", DisplayName: "Kiro", DefaultDirs: []string{".kiro/sessions/cli"}},
	{Type: "kiro-ide", DisplayName: "Kiro IDE", DefaultDirs: []string{
		".kiro-ide/sessions",
	}},
	{Type: "cortex", DisplayName: "Cortex", DefaultDirs: []string{
		".cortex/sessions",
	}},
	{Type: "hermes", DisplayName: "Hermes", DefaultDirs: []string{".hermes/sessions"}},
	{Type: "warp", DisplayName: "Warp", DefaultDirs: []string{".warp"}},
	{Type: "positron", DisplayName: "Positron", DefaultDirs: []string{".positron"}},
}

// ResolveAgentDirs returns the effective directories for an agent.
// Expands $HOME and filters to directories that exist.
func ResolveAgentDirs(def AgentDef) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var dirs []string
	for _, rel := range def.DefaultDirs {
		abs := filepath.Join(home, rel)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			dirs = append(dirs, abs)
		}
	}
	return dirs
}
