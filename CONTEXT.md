# SessionConflux

A single binary that syncs AI agent conversation sessions across machines (via Feishu Drive or SSH/SFTP) and serves a local web UI for browsing. Available as a CLI, a local web server, or a macOS desktop app.

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
Local JSON file (`~/.session-conflux/sync-state.json`) tracking each session's last-synced file size and mtime for change detection.
_Avoid_: Cache, checkpoint, progress file

**Agent Directory**:
The filesystem path where an AI agent stores its session JSONL files (e.g. `~/.claude/projects/`).
_Avoid_: Session folder, chat directory

**Transport**:
The pluggable storage backend that moves files between machines. Currently supports Feishu Drive and SSH/SFTP. All upper layers use the Transport interface.
_Avoid_: Backend driver, connector, adapter

**Cloud Sync**:
The cross-machine sync system: uploads local session changes to remote storage, downloads remote sessions to local agent directories. In daemon mode this runs on a schedule (upload then download). Powered by SessionConflux.
_Avoid_: Backup, replicate

**File Sync**:
The local filesystem watcher that discovers new or changed session JSONL files and indexes them into SQLite. Runs automatically during `serve`. An internal implementation detail — users don't invoke it directly.
_Avoid_: Indexing, ingestion, scanning

**Upload**:
One-shot command that scans local agent directories and pushes changed sessions to the configured transport (baseline bundle on first run, then individual compressed files).
_Avoid_: Push, send

**Download**:
One-shot command that retrieves all sessions from remote storage and writes them to local agent directories under `_synced/{hostname}/` so File Sync can discover and attribute them by source machine.
_Avoid_: Fetch, retrieve, pull

**Daemon**:
Background mode that periodically runs Cloud Sync (upload then download) without user intervention. Available via CLI (`session-conflux sync`) or configured through the Web UI settings page.
_Avoid_: Service, background worker, watcher

**Serve Mode**:
The `session-conflux serve` command: starts the local web UI at `http://127.0.0.1:8080`, runs File Sync to index local sessions into SQLite, and optionally starts the Cloud Sync daemon.
_Avoid_: Web server, backend, app server

**Desktop App**:
A macOS `.app` bundle (Tauri WebView) that wraps `session-conflux serve` as an internal sidecar. Provides a native menu bar experience without a terminal. Same product, different launch method.
_Avoid_: Native app, Electron app, wrapper

## Config

Unified TOML at `~/.session-conflux/config.toml`. Cloud Sync settings live under the `[sync]` and `[sync.transport]` blocks; all other keys belong to the local browsing engine.

Old `~/.session-conflux/config.toml` (pre-unification) and `~/.agentsview/config.toml` are auto-migrated on first startup.

## Relationships

- One **Session** produces one compressed `.jsonl.zst` file on remote storage
- The folder hierarchy (`hostname/{baseline,incremental}/`) on the configured transport serves as the index — no separate manifest file
- A **State File** tracks last-uploaded file size per **Session**
- Each **Agent Directory** contains zero or more local **Sessions**
- **Upload** reads local **Agent Directories** and writes to the configured transport
- **Download** reads from the configured transport and writes to local **Agent Directories** under `_synced/{hostname}/` for File Sync to index
- **File Sync** discovers local **Sessions** and indexes them into SQLite; it also picks up downloaded sessions from `_synced/{hostname}/` and tags them with the source machine
- **Cloud Sync** and **File Sync** are independent systems that happen to share the same binary and config file
- The single `session-conflux` binary provides all commands: `serve` starts the browser UI, `upload`/`download`/`sync` operate Cloud Sync, `setup` configures the transport
- The **Desktop App** launches `session-conflux serve` as a sidecar process and wraps its web UI in a Tauri WebView

## Example dialogue

> **Dev:** "How does download know which sessions are available?"

> **Domain expert:** "It calls ListFiles on the remote folder tree — `SessionConflux/{hostname}/incremental/` and `SessionConflux/{hostname}/baseline/` — and returns every `.jsonl.zst` file found. The path encodes hostname, agent, and session ID."

## Flagged ambiguities

- "sync" was used to mean both upload-only and bidirectional — resolved: Cloud Sync is bidirectional. Upload-only and download-only are separate commands.
- "archive" was used to mean both the compressed JSONL file and the manifest — resolved: Baseline is the compressed bundle, no manifest exists.
- "sync" in agentsview vs "sync" in session-conflux — resolved: File Sync (local, automatic) vs Cloud Sync (cross-machine, user-invoked).
