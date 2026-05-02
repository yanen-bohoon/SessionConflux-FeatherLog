# Feishu Drive as cross-machine session transport

SessionConflux v2 uses Feishu Drive (not Wiki Docx, not a self-hosted server) as the transport layer for syncing AI agent sessions across machines. Sessions are stored as JSONL compressed archives in a hierarchical folder structure, indexed by a single `manifest.json`. AgentsView continues to read from local SQLite — Feishu is invisible to it.

**Why:** The previous architecture (Feishu Wiki Docx blocks) was human-readable at the cost of being machine-unparseable. Two options were considered: self-hosted PostgreSQL (requires a server — defeats the no-cloud-server goal) and Feishu Drive file storage (zero infrastructure, accessible from any machine with the Feishu API). Drive was chosen because it provides cross-machine file transfer without running a server, and the 10GB free tier covers years of session data.
