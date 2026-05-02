# SessionConflux (v2)

A Go CLI tool that syncs AI agent conversation sessions across machines via Feishu Drive, consumed by AgentsView for local browsing.

## Language

**Session**:
An AI agent conversation record, stored as a JSONL file on disk.
_Avoid_: Chat, conversation log, transcript

**Manifest**:
A single JSON index file (`manifest.json`) at the root of Feishu Drive, listing every synced session's metadata and file token.
_Avoid_: Index, catalog, registry

**State File**:
Local JSON file (`~/.session-conflux/state.json`) tracking each session's last-synced message index.
_Avoid_: Cache, checkpoint, progress file

**Agent Directory**:
The filesystem path where an AI agent stores its session JSONL files (e.g. `~/.claude/projects/`).
_Avoid_: Session folder, chat directory

**Sync**:
The bidirectional process of pushing local session updates to Feishu Drive and pulling remote sessions to local agent directories.
_Avoid_: Upload, backup, replicate

**Pull**:
Manual, on-demand download of remote sessions from Feishu Drive to local agent directories.
_Avoid_: Fetch, retrieve

**Daemon**:
Background mode that periodically syncs to and from Feishu Drive without user intervention.
_Avoid_: Service, background worker, watcher

## Relationships

- One **Session** produces one compressed archive on Feishu Drive
- One **Manifest** indexes all **Sessions** across all computers
- A **State File** tracks sync progress per **Session**
- Each **Agent Directory** contains zero or more local **Sessions**
- **Sync** reads local **Agent Directories** and writes to Feishu Drive, or reads from Feishu Drive and writes to local **Agent Directories**
- A downloaded remote **Session** is placed into the corresponding **Agent Directory** for AgentsView to discover

## Example dialogue

> **Dev:** "When I pull a remote Session from Feishu Drive, does it appear in AgentsView immediately?"
> **Domain expert:** "Yes — the tool writes the JSONL into the Agent Directory, and AgentsView's fsnotify watcher detects it and indexes it into SQLite within seconds."

## Flagged ambiguities

- "sync" was used to mean both upload-only and bidirectional — resolved: Sync is bidirectional. Upload-only and download-only are called push and pull.
- "archive" was used to mean both the compressed JSONL file and the manifest — resolved: Archive is the compressed JSONL file, Manifest is the index.
