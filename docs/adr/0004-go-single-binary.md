# Go single binary over Python

SessionConflux v2 is written in Go (was Python). It compiles to a single zero-dependency binary for macOS, Windows, and Linux. Feishu Drive API calls use Go's `net/http` directly — no SDK dependency.

**Why:** Three reasons. First, AgentsView's 21 session parsers are Go — rewriting them in Python is a non-starter. Second, a single binary means installation is trivial across platforms (download, run, done). Third, `lark-cli` was considered as an upload dependency but rejected — it's a full CLI tool pulling in unnecessary weight. The handful of Feishu API calls needed (auth, upload, download) are ~300 lines of Go.
