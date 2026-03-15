# Stability Wrap-Up (2026-03-15)

This note closes out the March sync/server hardening pass. The work landed directly on `main` in a series of focused commits and is now the baseline behavior of the repo.

## What Changed

### Sync correctness

- Fixed stale `today_index_ref` carry-over when later task updates changed `sr` or `schedule` without sending a fresh `tir`.
- Added a schema v4 migration that invalidates old local cache state and forces a full resync, because previously corrupted `sr`/`tir` rows could not be repaired in place.
- Preserved real source `server_index` values when flattening multi-entity history slots.
- Hid soft-deleted tasks from direct UUID lookups.
- Standardized task movement detection on UTC day boundaries.
- Added semantic task assignment changes for project and area moves.

### Server hardening

- Added a default HTTP client timeout.
- Hardened recurrence parsing against malformed repeat payloads that previously panicked.
- Replaced fragile string-based 409 handling with typed HTTP status errors.
- Enforced request body size limits on JSON endpoints.
- Startup now fails if the initial sync fails, instead of serving an empty or stale snapshot.
- Added graceful HTTP shutdown on `SIGINT` and `SIGTERM`.
- Escaped account emails in account URLs.
- Stopped discarding MCP JSON marshal failures; these now surface as tool errors.
- Bounded debug request/response dumps so logs do not print arbitrarily large payloads.

### Read/API behavior

- Added optional pagination (`limit`, `offset`) to:
  - REST read endpoints for inbox, today, projects, areas, and tags
  - MCP list tools for today, inbox, all tasks, projects, areas, tags, project tasks, and area tasks
- Added local purge of already soft-deleted rows after sync to stop unbounded cache growth.

## Deployment Notes

- Upgrading from an older database may trigger a one-time local cache wipe and full rebuild on open because of the schema v4 migration.
- The server will now exit on startup if Things Cloud cannot be synced successfully.
- Read clients that want stable paging should pass explicit `limit` and `offset` values instead of assuming whole-library responses forever.

## Commits

- `c4316de` Reset stale Today refs and resync corrupted caches
- `4216f10` Harden startup and MCP JSON responses
- `ad1a2e4` Bound debug dumps and name repeat sentinel
- `64bb1f7` Gracefully shut down the HTTP server
- `b552ec1` Paginate state queries and purge deleted rows
