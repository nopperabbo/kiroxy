# BUILD_LOG.md — kiroxy v0.1.0-mvp construction log

Append-only. One entry per milestone.

---

## M1 — Fork & Scaffold  (2026-05-11 19:28 UTC)
- Hours: 1.0 (on budget)
- Commit: pending (will be first commit on main)
- Gate: **green**
- Verification output:
  ```
  GET /healthz → 200 {"started_at":"2026-05-11T19:28:02Z","status":"ok","uptime_s":0,"version":"0.1.0-mvp"}
  kiroxy version → 0.1.0-mvp
  kiroxy bogus   → exit 1, "unknown subcommand"
  go build ./... → 0
  go vet ./...   → 0
  gofmt -l .     → empty
  go test ./...  → no test files (expected pre-M2)
  ```
- Decisions actually taken:
  - D1: Docker **dropped** from MVP gate (user has no docker). Will revisit in Phase 2. `Dockerfile` scheduled for M8 docs.
  - D5: Redis **dropped** (single-user, SQLite is enough).
  - Module path: `local/kiroxy` (not publishable).
  - Port: 8787 default. LogLevel: info default.
- Files created:
  - `LICENSE`, `NOTICE`, `README.md`, `CHANGELOG.md`, `BACKLOG.md`, `BUILD_LOG.md`, `.gitignore`, `.env.example`
  - `go.mod` (module local/kiroxy, go 1.26, zero deps)
  - `cmd/kiroxy/main.go` (entry + subcommand dispatch + awaitShutdown pattern from kirocc)
  - `internal/config/config.go` (env+flag parsing, no external deps)
  - `internal/server/server.go` (minimal mux + /healthz)
- Surprises: none. One comment-hook nudge — trimmed one redundant method comment; kept Go-convention-mandated exported-symbol doc comments.
- Next: M2 — Kirocc Converter Graft

---
