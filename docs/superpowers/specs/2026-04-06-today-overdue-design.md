# Today View: Include Overdue Tasks

## Context

The `things_list_today` MCP tool only returns tasks scheduled for today's exact date. In the real Things 3 app, the Today view also shows overdue tasks — tasks scheduled for a past date that were never completed. This mismatch means the MCP tool under-reports what's actually in a user's Today view.

The current workaround is to call both `things_list_today` and `things_list_all_tasks` (filtering for `scheduled_for < today AND status == "open"`), then merge results. This is fragile, requires two tool calls, and pushes classification logic onto the LLM consumer.

### How Things 3 Determines "Today"

From the SDK's reverse-engineering (`docs/client-side-bugs.md`), the wire fields are:

| `st` (schedule) | `sr` / `tir` date | Things UI view |
|------------------|-------------------|----------------|
| 0 (Inbox)        | null              | Inbox          |
| 1 (Anytime)      | today's date      | **Today**      |
| 1 (Anytime)      | past date         | **Today** (overdue) |
| 1 (Anytime)      | null              | Anytime        |
| 2 (Someday)      | future date       | Upcoming       |
| 2 (Someday)      | null              | Someday        |

The missing row (past date + `schedule=1`) is the bug.

## Problem

`TasksInToday()` in `sync/state.go` uses this SQL:

```sql
WHERE type = 0 AND schedule = 1
AND (
    (scheduled_date >= todayMidnight AND scheduled_date < tomorrowMidnight)
    OR (today_index_ref >= todayMidnight AND today_index_ref < tomorrowMidnight)
)
```

The `>= todayMidnight` lower bound excludes all past dates. A task with `scheduled_date = yesterday` and `schedule = 1` falls through to `TasksInAnytime` instead.

The same narrow logic exists in `taskLocationAt()` in `sync/detect.go`, which uses `isTodayAt()` (exact calendar-day match) to classify a task's location.

## Approach: Fix the SDK's Core Definition of Today

Fix the SQL queries and change detection together so "Today" consistently means "schedule=1 with any non-null date that is today or earlier."

### Piece A: SQL Query Changes (`sync/state.go`)

**`TasksInToday` (lines 160-190)**

Before:
```go
todayUnix, tomorrowUnix := currentUTCDayBounds()

query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
    AND (
        (scheduled_date >= ? AND scheduled_date < ?)
        OR (today_index_ref >= ? AND today_index_ref < ?)
    ) AND deleted = 0`
args := []any{todayUnix, tomorrowUnix, todayUnix, tomorrowUnix}
```

After:
```go
_, tomorrowUnix := currentUTCDayBounds()

query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
    AND (
        (scheduled_date IS NOT NULL AND scheduled_date < ?)
        OR (today_index_ref IS NOT NULL AND today_index_ref < ?)
    ) AND deleted = 0`
args := []any{tomorrowUnix, tomorrowUnix}
```

The lower bound `>= todayMidnight` is removed. The `IS NOT NULL` guard ensures tasks with no date stay in Anytime. Any task with a date before tomorrow (today or any past date) is classified as Today.

**`TasksInAnytime` (lines 192-212)**

Mirror change so the two queries remain exact complements:

Before:
```go
todayUnix, tomorrowUnix := currentUTCDayBounds()

query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
    AND NOT (
        (scheduled_date IS NOT NULL AND scheduled_date >= ? AND scheduled_date < ?)
        OR (today_index_ref IS NOT NULL AND today_index_ref >= ? AND today_index_ref < ?)
    ) AND deleted = 0`
args := []any{todayUnix, tomorrowUnix, todayUnix, tomorrowUnix}
```

After:
```go
_, tomorrowUnix := currentUTCDayBounds()

query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
    AND NOT (
        (scheduled_date IS NOT NULL AND scheduled_date < ?)
        OR (today_index_ref IS NOT NULL AND today_index_ref < ?)
    ) AND deleted = 0`
args := []any{tomorrowUnix, tomorrowUnix}
```

**Update doc comment** on `TasksInToday`:
```go
// TasksInToday returns tasks in the Today view. A task appears in Today when
// schedule=1 (started/anytime) AND either sr (scheduled_date) or tir
// (today_index_ref) is set to today's date or any past date (overdue).
```

### Piece B: Change Detection (`sync/detect.go`)

**New helper function** — `isPastOrTodayAt`:

```go
func isPastOrTodayAt(t *time.Time, now time.Time) bool {
    if t == nil {
        return false
    }
    nowUTC := now.UTC()
    tomorrow := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day()+1, 0, 0, 0, 0, time.UTC)
    return t.UTC().Before(tomorrow)
}
```

**Update `taskLocationAt`** (lines 138-157):

Before:
```go
case things.TaskScheduleAnytime:
    if isTodayAt(t.ScheduledDate, now) || isTodayAt(t.TodayIndexReference, now) {
        return LocationToday
    }
    return LocationAnytime
```

After:
```go
case things.TaskScheduleAnytime:
    if isPastOrTodayAt(t.ScheduledDate, now) || isPastOrTodayAt(t.TodayIndexReference, now) {
        return LocationToday
    }
    return LocationAnytime
```

`isTodayAt` and `isToday` remain in the codebase — they may be used elsewhere and are still valid helpers.

**Impact on change events:**

`taskLocationAt` is called by `detectTaskChanges` (line 96) to compare old vs new task locations during sync. The change only affects classification, not event generation logic. Scenarios:

- Task edited but dates unchanged: old and new both classify as Today (or both Anytime). No spurious `TaskMovedTo*` event.
- Task newly scheduled for a past date (unusual): would now correctly emit `TaskMovedToToday` instead of `TaskMovedToAnytime`.
- Existing overdue tasks with no cloud changes: `detectTaskChanges` isn't called (no new Item arrived), so no events generated.

`syncutil.BuildDailySummary()` and `CountMoves()` consume stored change events from the `change_log` table. Past events are unaffected. Future events will correctly reflect the updated classification.

`cmd/thingsync`'s `getTaskLocation()` uses `TodayIndex != 0` — a completely separate mechanism. It is not affected by this change.

## Files Changed

| File | Change |
|------|--------|
| `sync/state.go` | `TasksInToday`: remove `>= todayMidnight` bound, add `IS NOT NULL`. `TasksInAnytime`: mirror exclusion. Update doc comments. |
| `sync/detect.go` | Add `isPastOrTodayAt` helper. Update `taskLocationAt` to use it for the `TaskScheduleAnytime` branch. |
| `sync/integration_test.go` | Update `TestTasksInTodayWithTIR` expectations. Add `TestTasksInTodayIncludesOverdue`. Add `TestTasksInAnytimeExcludesOverdue`. |
| `sync/detect_test.go` | Add `taskLocation` test cases for past dates. Add change detection test for overdue transition. |

## Test Plan

All tests use `syncer.saveTask()` with temp SQLite databases. No real Things account or network access required.

### Updated Tests

**1. `TestTasksInTodayWithTIR` (sync/integration_test.go)**

This test creates 8 tasks with various date combinations. Changes:

- Move `"both-old"` (sr=yesterday, tir=yesterday) from `notExpected` to `expected` — both dates are past, so this task is now overdue/Today
- Add new task `"sr-only-overdue"` with `schedule=1, sr=3 days ago, no tir` — add to `expected`
- `"no-dates"` stays in `notExpected` (no date = Anytime)
- `"inbox-with-tir"` stays in `notExpected` (schedule=0 = Inbox regardless of dates)

Updated expectations:
```go
expected := []string{
    "sr-only", "tir-only", "both-today",
    "sr-old-tir-today", "sr-today-tir-old",
    "both-old",        // was notExpected — now overdue/Today
    "sr-only-overdue", // new test case
}
notExpected := []string{"no-dates", "inbox-with-tir"}
```

### New Tests

**2. `TestTasksInTodayIncludesOverdue` (sync/integration_test.go)**

Focused test for the overdue scenario with edge cases:

| Task UUID | schedule | sr | tir | Expected in Today? |
|-----------|----------|----|-----|--------------------|
| `overdue-1d` | 1 | yesterday | nil | Yes |
| `overdue-7d` | 1 | 7 days ago | nil | Yes |
| `overdue-30d` | 1 | 30 days ago | nil | Yes |
| `overdue-tir` | 1 | nil | 3 days ago | Yes |
| `overdue-completed` | 1 (completed) | yesterday | nil | No (status filter) |
| `overdue-inbox` | 0 | yesterday | nil | No (wrong schedule) |
| `overdue-someday` | 2 | yesterday | nil | No (wrong schedule) |
| `today-exact` | 1 | today | nil | Yes (regression guard) |
| `anytime-no-date` | 1 | nil | nil | No (Anytime) |

**3. `TestTasksInAnytimeExcludesOverdue` (sync/integration_test.go)**

Verify overdue tasks don't leak into Anytime:

| Task UUID | schedule | sr | tir | Expected in Anytime? |
|-----------|----------|----|-----|----------------------|
| `anytime-clean` | 1 | nil | nil | Yes |
| `overdue-sr` | 1 | yesterday | nil | No (now in Today) |
| `overdue-tir` | 1 | nil | 3 days ago | No (now in Today) |
| `today-sr` | 1 | today | nil | No (in Today) |

**4. `TestTaskLocationOverdue` (sync/detect_test.go)**

Test `taskLocationAt` with past dates:

```go
t.Run("anytime schedule with past date", func(t *testing.T) {
    yesterday := time.Now().AddDate(0, 0, -1)
    task := &things.Task{Schedule: things.TaskScheduleAnytime, ScheduledDate: &yesterday}
    loc := taskLocation(task)
    // expect LocationToday
})

t.Run("anytime schedule with past tir", func(t *testing.T) {
    threeDaysAgo := time.Now().AddDate(0, 0, -3)
    task := &things.Task{Schedule: things.TaskScheduleAnytime, TodayIndexReference: &threeDaysAgo}
    loc := taskLocation(task)
    // expect LocationToday
})

t.Run("anytime schedule with far past date", func(t *testing.T) {
    monthAgo := time.Now().AddDate(0, -1, 0)
    task := &things.Task{Schedule: things.TaskScheduleAnytime, ScheduledDate: &monthAgo}
    loc := taskLocation(task)
    // expect LocationToday
})
```

**5. `TestDetectOverdueTransition` (sync/detect_test.go)**

Test that change detection correctly classifies an overdue task:

```go
t.Run("task with past date detected as today", func(t *testing.T) {
    yesterday := time.Now().AddDate(0, 0, -1)
    old := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleInbox}
    new := &things.Task{UUID: "t1", Title: "Task", Schedule: things.TaskScheduleAnytime, ScheduledDate: &yesterday}
    changes := detectTaskChanges(old, new, 1, now)
    // expect TaskMovedToToday (not TaskMovedToAnytime)
})
```

### Existing Tests — Expected to Pass Without Changes

These tests should continue passing as-is (regression guard):

- `TestTasksInTodayWithTIR` — after updating expectations per above
- `TestStateQueries/"TasksInToday includes tir-only tasks"` — test data has no past-dated `schedule=1` tasks, count stays at 1
- `TestStateQueries/"TasksInAnytime returns undated anytime tasks"` — `"anytime-1"` has no dates, stays in Anytime
- `TestStateQueries/"TasksInSomeday"` — `schedule=2` tasks unaffected
- `TestStateQueries/"TasksInUpcoming"` — `schedule=2` tasks unaffected
- All existing `detect_test.go` location tests — `isTodayAt` cases still pass because today's date is both "today" and "past-or-today"

### Local Verification

```bash
# Run all sync package tests (no network needed)
go test -v ./sync/...

# Run specific new tests
go test -v -run TestTasksInTodayIncludesOverdue ./sync/
go test -v -run TestTasksInAnytimeExcludesOverdue ./sync/
go test -v -run TestTaskLocationOverdue ./sync/
go test -v -run TestDetectOverdueTransition ./sync/

# Full suite
go test -v ./...
```

### End-to-End Verification (Needs Real Account)

A dummy Things 3 account exists for local testing. Credentials are in `.env.test` (git-ignored).

**Important caveats discovered during testing:**

- **`parseWhen()` normalizes past dates to today** (`server/write.go` line 521): You cannot create an overdue task via the API — `things_create_task` with `when: "yesterday"` will schedule the task for today. Overdue tasks only exist when a previously-scheduled task's date passes without completion.
- **Pre-existing test failure**: `TestIntegration/query_change_log` fails (`expected 4 changes after index 0, got 0`). This is unrelated to the today/overdue fix and exists on main before any changes.
- **Go path on macOS**: May need `export PATH="/opt/homebrew/bin:$PATH"` if Homebrew Go isn't in the default path.

**Start the local server:**

```bash
# Load test credentials (or export manually)
export $(grep -v '^#' .env.test | xargs)
go run ./server/
```

**Verify the fix (before/after):**

1. Call `things_list_today` — check for any naturally overdue tasks (tasks scheduled before today that were never completed)
2. Call `things_list_anytime` — overdue tasks should NOT appear here after the fix
3. Run smoke test: `./tests/test-smoke.sh http://localhost:9090` — should pass 11/11

**To create a testable overdue task:**

Since the API normalizes past dates, create a task scheduled for today, then wait 24+ hours before testing. Or use an account that already has overdue tasks (the dummy account has task "Today is here!" scheduled for 2026-04-08, which will be overdue after that date).

## Out of Scope

- **Upcoming tasks whose start date has passed** (`schedule=2` + past date): These currently fall into Someday. Whether they should surface in Today is a separate question with different trade-offs (it changes the meaning of `schedule=2`).
- **`cmd/thingsync`'s `getTaskLocation()`**: Uses `TodayIndex != 0`, a separate mechanism. Aligning it is a larger refactor.
- **`state/memory` package**: Has no today/anytime concept. Not affected.
- **MCP tool description update**: The `things_list_today` description says "List tasks scheduled for today" — could optionally be updated to mention overdue tasks, but not required for correctness.

## Implementation Order

1. **Commit 1**: Piece A — SQL query changes in `sync/state.go` + all query-level tests
2. **Commit 2**: Piece B — `isPastOrTodayAt` helper and `taskLocationAt` update in `sync/detect.go` + detection-level tests

This separation allows Piece B to be reverted independently if change detection behavior causes unexpected issues downstream.
