# Code Review (2026-02-17)

## Scope
Reviewed crash-sensitive and recently modified paths:
- `server/main.go`
- `server/write.go`
- `client.go`
- `cmd/things-cli/main.go`
- `cmd/dump-history/main.go`

## Findings

### [P0] `things-cli` currently fails to compile due stale call site after function signature change
- File: `cmd/things-cli/main.go:1178`
- Evidence: `newTaskCreatePayload` now returns `(TaskCreatePayload, error)`, but `buildBatchCreate` still assigns a single value.
- Impact: `go test ./...` fails; CLI batch path is unusable.

### [P0] `dump-history` tool fails to compile due incorrect `json.Indent` destination type
- File: `cmd/dump-history/main.go:87`
- Evidence: `json.Indent` expects `*bytes.Buffer`, but code passes `*json.RawMessage`.
- Impact: `go test ./...` fails and debug utility cannot be built.

### [P1] Production server enables full HTTP debug dumps unconditionally
- Files: `server/main.go:170`, `client.go:125`, `client.go:126`, `client.go:133`
- Evidence: `client.Debug = true` forces request/response dumps, including headers and bodies.
- Impact: credentials and sensitive task contents can be written to logs (`Authorization: Password ...`, notes, metadata).

### [P1] Destructive debug history endpoints are reachable whenever `API_KEY` is unset
- Files: `server/main.go:249`, `server/main.go:267`, `server/main.go:126`, `server/main.go:129`
- Evidence: debug delete endpoint is protected by `authMiddleware`, which allows all requests when `API_KEY` is empty.
- Impact: unauthenticated callers can delete history keys in deployments without `API_KEY`, causing data loss/outage.

### [P1] Batch write path still lacks strict UUID validation for user-provided IDs
- Files: `cmd/things-cli/main.go:1235`, `cmd/things-cli/main.go:1239`, `cmd/things-cli/main.go:1242`, `cmd/things-cli/main.go:1249`, `cmd/things-cli/main.go:1252`, `cmd/things-cli/main.go:1263`, `cmd/things-cli/main.go:1293`, `cmd/things-cli/main.go:1299`, `cmd/things-cli/main.go:1305`, `cmd/things-cli/main.go:1311`
- Evidence: batch handlers check only for non-empty strings in several operations and directly serialize IDs into commit payloads.
- Impact: malformed/non-Base58 IDs can be committed and may reproduce the same client-side sync crash signature (`EXC_BREAKPOINT` in `Base.framework` decode path).

### [P2] Batch move/edit logic writes epoch-zero scheduling dates for "anytime" transitions
- Files: `cmd/things-cli/main.go:1242`, `cmd/things-cli/main.go:1256`, `cmd/things-cli/main.go:1296`, `cmd/things-cli/main.go:1302`, `cmd/things-cli/main.go:1308`
- Evidence: uses `Schedule(1, 0, 0)` instead of `nil` date fields for anytime semantics.
- Impact: inconsistent wire semantics; risks unexpected placement/sorting behavior and protocol drift.

### [P2] Write-path payload logging can leak private task content
- File: `server/write.go:429`, `server/write.go:430`
- Evidence: logs full serialized write envelope including note text and metadata.
- Impact: sensitive user content is persisted to logs and observability systems.

## Reproduction Note
Current repository build status from `go test ./...`:
- Fails in `cmd/dump-history` and `cmd/things-cli` for the two P0 findings above.

## Reviewer Notes
No fixes were applied in this review. Findings only.
