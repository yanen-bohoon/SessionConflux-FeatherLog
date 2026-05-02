# JSONL compressed archives over SQLite dumps

Session data uploaded to Feishu Drive is stored as per-session normalized JSONL files (compressed with zstd), not as SQLite database dumps.

**Why:** SQLite dumps are download-and-go but conflict-prone when multiple machines append to the same database. JSONL archives are append-only by nature — each machine uploads its own session files independently. This also enables incremental sync: only sessions with new messages (tracked by state file) are re-uploaded, instead of transferring the entire database every time. The trade-off is that downloaded sessions must be re-parsed locally, but AgentsView already parses JSONL on discovery, so this happens automatically.
