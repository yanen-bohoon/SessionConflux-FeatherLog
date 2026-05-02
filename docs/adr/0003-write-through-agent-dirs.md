# Write-through to agent directories over direct DB injection

Remote sessions pulled from Feishu Drive are written as JSONL files into the corresponding agent directory (e.g. `~/.claude/projects/...`). AgentsView discovers them via its existing fsnotify watcher and indexes them into SQLite.

**Why:** Two alternatives were rejected. Direct SQLite writes (importing AgentsView's `internal/db/` package) couples the tool to AgentsView's internal schema and user_version migration logic — fragile and high-maintenance. API compatibility (re-implementing AgentsView's REST endpoints) requires ongoing alignment with upstream changes. Writing JSONL to agent directories requires zero knowledge of AgentsView internals and works with any version of AgentsView, past or future. The cost is disk duplication (JSONL + SQLite), but JSONL compresses well and the duplicate is the same format already on disk from local sessions.
