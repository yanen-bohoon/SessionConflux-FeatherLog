# SessionConflux

Sync AI agent conversation sessions across machines via [Feishu Drive](https://open.feishu.cn/) — no cloud server required.

Pairs with [AgentsView](https://github.com/wesm/agentsview) for local browsing. Uploads session JSONL files to Feishu Drive, downloads them on other machines, and AgentsView discovers them automatically.

## Install

Download the binary for your platform from [Releases](https://github.com/yanen-bohoon/SessionConflux-FeatherLog/releases).

Or build from source:

```
git clone https://github.com/yanen-bohoon/SessionConflux-FeatherLog.git
cd SessionConflux-FeatherLog
make build
```

## Quick start

```sh
# 1. Configure Feishu credentials
session-conflux config

# 2. List local sessions
session-conflux list

# 3. Upload to Feishu Drive
session-conflux upload

# 4. On another machine, download all
session-conflux download --all

# 5. Or run the daemon for daily auto-sync
session-conflux sync
```

## Commands

| Command | Description |
|---------|-------------|
| `config` | Interactive Feishu credential setup |
| `list` | List all local AI agent sessions |
| `upload` | Upload changed sessions to Feishu Drive |
| `download` | Download sessions from Feishu Drive (`--all` / `--session <key>`) |
| `sync` | Daemon mode — auto-sync daily at scheduled time |
| `version` | Show version |

## Supported agents

Claude Code, Codex, Gemini CLI, Copilot, Cursor, OpenCode, OpenHands, Amp, Zencoder, iFlow, VS Code Copilot, Pi, OpenClaw, Kimi, Claude.ai, ChatGPT, Kiro, Kiro IDE, Cortex, Hermes, Warp, Positron.

## How it works

1. **Scan** — discovers JSONL session files from 21 AI agents
2. **Compress** — zstd-compresses raw JSONL
3. **Upload** — pushes to Feishu Drive in `computer/agent/session_id.jsonl.zst` hierarchy
4. **Manifest** — maintains `manifest.json` on Drive as the session index
5. **Download** — pulls remote sessions and writes to agent directories
6. **AgentsView** — fsnotify picks up new files and indexes them into local SQLite

## Configuration

`~/.session-conflux/config.toml`:

```toml
[feishu]
app_id = "cli_xxx"
app_secret = "xxx"

[sync]
schedule = "02:00"
direction = "both"

[agents]
exclude = ["warp"]

[compression]
level = 3
```

## License

MIT
