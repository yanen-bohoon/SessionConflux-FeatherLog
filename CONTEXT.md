# SessionConflux (v2)

A Go CLI tool that syncs AI agent conversation sessions across machines via Feishu Drive or SSH/SFTP, consumed by AgentsView for local browsing.

## Language

**Session**:
An AI agent conversation record, stored as a JSONL file on disk.
_Avoid_: Chat, conversation log, transcript

**Baseline**:
A compressed tar.zst bundle of all sessions on a machine, uploaded on first sync or when no baseline exists on Drive. May be split into 19MB parts for Feishu's upload limit.
_Avoid_: Snapshot, full backup, archive

**Incremental**:
A single compressed session JSONL file uploaded when its size has changed since the last upload. Stored per-hostname under `incremental/`.
_Avoid_: Delta, diff, patch

**State File**:
Local JSON file (`~/.session-conflux/state.json`) tracking each session's last-synced file size and mtime for change detection. Both come from a single `stat` call — zero extra I/O.
_Avoid_: Cache, checkpoint, progress file

**Agent Directory**:
The filesystem path where an AI agent stores its session JSONL files (e.g. `~/.claude/projects/`).
_Avoid_: Session folder, chat directory

**Transport**:
The pluggable storage backend that moves files between machines. Currently supports Feishu Drive and SSH/SFTP. All upper layers use the Transport interface — they never call a specific backend directly.
_Avoid_: Backend driver, connector, adapter

**Sync**:
The bidirectional process of pushing local session updates to remote storage and pulling remote sessions to local agent directories. In daemon mode this runs upload then download.
_Avoid_: Backup, replicate

**Upload**:
One-shot command that scans local agent directories and pushes changed sessions to the configured transport (baseline bundle on first run, then individual compressed files).
_Avoid_: Push, send

**Download**:
One-shot command that retrieves all sessions from remote storage and writes them to local agent directories under `_synced/{hostname}/`. Re-downloads the baseline bundle plus every incremental file.
_Avoid_: Fetch, retrieve, pull

**Daemon**:
Background mode that periodically syncs to and from remote storage without user intervention.
_Avoid_: Service, background worker, watcher

## Relationships

- One **Session** produces one compressed `.jsonl.zst` file on remote storage
- The folder hierarchy (`hostname/{baseline,incremental}/`) on the configured transport serves as the index — no separate manifest file
- A **State File** tracks last-uploaded file size per **Session**
- Each **Agent Directory** contains zero or more local **Sessions**
- **Upload** reads local **Agent Directories** and writes to the configured transport
- **Download** reads from the configured transport and writes to local **Agent Directories** under `_synced/{hostname}/` for AgentsView to discover with correct machine attribution
- SessionConflux distributes a **modified AgentsView** that understands `_synced/{hostname}/` path classification and per-session machine tags. Upstream AgentsView does not include these patches and will not correctly attribute synced sessions.

## Example dialogue

> **Dev:** "How does download know which sessions are available?"

> **Domain expert:** "It calls ListFiles on the remote folder tree — `SessionConflux/{hostname}/incremental/` and `SessionConflux/{hostname}/baseline/` — and returns every `.jsonl.zst` file found. The path encodes hostname, agent, and session ID."

## Flagged ambiguities

- "sync" was used to mean both upload-only and bidirectional — resolved: Sync is bidirectional. Upload-only and download-only are separate commands.
- "archive" was used to mean both the compressed JSONL file and the manifest — resolved: Baseline is the compressed bundle, no manifest exists.
