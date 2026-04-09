# Today View: Include Overdue Tasks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `TasksInToday()` include overdue tasks (schedule=1 with a past date), matching the real Things 3 app's Today view.

**Architecture:** Two-piece fix. Piece A changes the SQL queries in `sync/state.go` so `TasksInToday` matches any date before tomorrow (not just today), and `TasksInAnytime` excludes those same tasks. Piece B adds an `isPastOrTodayAt` helper in `sync/detect.go` and updates `taskLocationAt` so change detection classifies overdue tasks as Today. Each piece gets its own commit.

**Tech Stack:** Go, SQLite, `go test`

**Spec:** `docs/superpowers/specs/2026-04-06-today-overdue-design.md`

**Prerequisites:** All commands assume Go is on PATH. If using Homebrew Go on macOS, run `export PATH="/opt/homebrew/bin:$PATH"` once at the start of the session.

**Known pre-existing failure:** `TestIntegration/query_change_log` fails before any changes (`expected 4 changes after index 0, got 0`). This is unrelated to the today/overdue fix. Any `go test` run that includes it will exit code 1. Verify it is the ONLY failing test when checking for regressions.

---

### Task 1: Write failing tests for `TasksInToday` overdue behavior

**Files:**
- Modify: `sync/integration_test.go`

These tests exercise the SQL query layer. They will fail against the current code because overdue tasks (past dates) are excluded from Today.

- [ ] **Step 1: Update `TestTasksInTodayWithTIR` expectations**

In `TestTasksInTodayWithTIR`, the task `"both-old"` has `sr=yesterday, tir=yesterday`. After our fix, this is overdue and belongs in Today. Also add a new overdue-only test task.

Add a new task after the `"both-old"` saveTask call (after line 276):

```go
	threeDaysAgo := today.Add(-3 * 24 * time.Hour)
	syncer.saveTask(&things.Task{
		UUID:          "sr-only-overdue",
		Title:         "SR Only Overdue",
		Status:        things.TaskStatusPending,
		Schedule:      things.TaskScheduleAnytime,
		ScheduledDate: &threeDaysAgo,
	})
```

Update the expected/notExpected slices. Replace:
```go
	expected := []string{"sr-only", "tir-only", "both-today", "sr-old-tir-today", "sr-today-tir-old"}
```
With:
```go
	expected := []string{"sr-only", "tir-only", "both-today", "sr-old-tir-today", "sr-today-tir-old", "both-old", "sr-only-overdue"}
```

Replace:
```go
	notExpected := []string{"both-old", "no-dates", "inbox-with-tir"}
```
With:
```go
	notExpected := []string{"no-dates", "inbox-with-tir"}
```

- [ ] **Step 2: Add `TestTasksInTodayIncludesOverdue` test**

Add this test after `TestTasksInTodayWithTIR`:

```go
func TestTasksInTodayIncludesOverdue(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	nowUTC := time.Now().UTC()
	today := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 12, 0, 0, 0, time.UTC)
	yesterday := today.Add(-24 * time.Hour)
	sevenDaysAgo := today.Add(-7 * 24 * time.Hour)
	thirtyDaysAgo := today.Add(-30 * 24 * time.Hour)
	threeDaysAgo := today.Add(-3 * 24 * time.Hour)

	syncer.saveTask(&things.Task{
		UUID: "overdue-1d", Title: "Overdue 1 day",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		ScheduledDate: &yesterday,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-7d", Title: "Overdue 7 days",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		ScheduledDate: &sevenDaysAgo,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-30d", Title: "Overdue 30 days",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		ScheduledDate: &thirtyDaysAgo,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-tir", Title: "Overdue via TIR",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		TodayIndexReference: &threeDaysAgo,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-completed", Title: "Overdue but completed",
		Status: things.TaskStatusCompleted, Schedule: things.TaskScheduleAnytime,
		ScheduledDate: &yesterday,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-inbox", Title: "Overdue but inbox",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleInbox,
		ScheduledDate: &yesterday,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-someday", Title: "Overdue but someday",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleSomeday,
		ScheduledDate: &yesterday,
	})
	syncer.saveTask(&things.Task{
		UUID: "today-exact", Title: "Today exact",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		ScheduledDate: &today,
	})
	syncer.saveTask(&things.Task{
		UUID: "anytime-no-date", Title: "Anytime no date",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
	})

	tasks, err := syncer.State().TasksInToday(QueryOpts{})
	if err != nil {
		t.Fatalf("TasksInToday failed: %v", err)
	}

	got := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		got[task.UUID] = true
	}

	expected := []string{"overdue-1d", "overdue-7d", "overdue-30d", "overdue-tir", "today-exact"}
	for _, uuid := range expected {
		if !got[uuid] {
			t.Errorf("expected task %q in Today, but not found", uuid)
		}
	}

	notExpected := []string{"overdue-completed", "overdue-inbox", "overdue-someday", "anytime-no-date"}
	for _, uuid := range notExpected {
		if got[uuid] {
			t.Errorf("task %q should NOT be in Today", uuid)
		}
	}

	if len(tasks) != len(expected) {
		t.Errorf("expected %d tasks in Today, got %d", len(expected), len(tasks))
	}
}
```

- [ ] **Step 3: Add `TestTasksInAnytimeExcludesOverdue` test**

Add this test immediately after `TestTasksInTodayIncludesOverdue`:

```go
func TestTasksInAnytimeExcludesOverdue(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	syncer, err := Open(dbPath, nil)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer syncer.Close()

	nowUTC := time.Now().UTC()
	today := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 12, 0, 0, 0, time.UTC)
	yesterday := today.Add(-24 * time.Hour)
	threeDaysAgo := today.Add(-3 * 24 * time.Hour)

	syncer.saveTask(&things.Task{
		UUID: "anytime-clean", Title: "Anytime clean",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-sr", Title: "Overdue SR",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		ScheduledDate: &yesterday,
	})
	syncer.saveTask(&things.Task{
		UUID: "overdue-tir", Title: "Overdue TIR",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		TodayIndexReference: &threeDaysAgo,
	})
	syncer.saveTask(&things.Task{
		UUID: "today-sr", Title: "Today SR",
		Status: things.TaskStatusPending, Schedule: things.TaskScheduleAnytime,
		ScheduledDate: &today,
	})

	tasks, err := syncer.State().TasksInAnytime(QueryOpts{})
	if err != nil {
		t.Fatalf("TasksInAnytime failed: %v", err)
	}

	got := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		got[task.UUID] = true
	}

	if !got["anytime-clean"] {
		t.Error("expected anytime-clean in Anytime, but not found")
	}

	shouldNotBeInAnytime := []string{"overdue-sr", "overdue-tir", "today-sr"}
	for _, uuid := range shouldNotBeInAnytime {
		if got[uuid] {
			t.Errorf("task %q should NOT be in Anytime (should be in Today)", uuid)
		}
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 task in Anytime, got %d", len(tasks))
	}
}
```

- [ ] **Step 4: Run new tests to verify they fail**

Run:
```bash
go test -v -run "TestTasksInTodayIncludesOverdue|TestTasksInAnytimeExcludesOverdue|TestTasksInTodayWithTIR" ./sync/
```

Expected: All three FAIL. `TestTasksInTodayIncludesOverdue` — overdue tasks missing from Today. `TestTasksInAnytimeExcludesOverdue` — overdue tasks incorrectly present in Anytime. `TestTasksInTodayWithTIR` — `both-old` and `sr-only-overdue` not found in Today.

---

### Task 2: Implement Piece A — SQL query changes in `sync/state.go`

**Files:**
- Modify: `sync/state.go:160-212`

- [ ] **Step 1: Update `TasksInToday` query (lines 160-171)**

Replace the doc comment and query:

```go
// TasksInToday returns tasks in the Today view. A task appears in Today when
// schedule=1 (started/anytime) AND either sr (scheduled_date) or tir
// (today_index_ref) falls on today's date.
func (st *State) TasksInToday(opts QueryOpts) ([]*things.Task, error) {
	todayUnix, tomorrowUnix := currentUTCDayBounds()

	query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
		AND (
			(scheduled_date >= ? AND scheduled_date < ?)
			OR (today_index_ref >= ? AND today_index_ref < ?)
		) AND deleted = 0`
	args := []any{todayUnix, tomorrowUnix, todayUnix, tomorrowUnix}
```

With:

```go
// TasksInToday returns tasks in the Today view. A task appears in Today when
// schedule=1 (started/anytime) AND either sr (scheduled_date) or tir
// (today_index_ref) is set to today's date or any past date (overdue).
func (st *State) TasksInToday(opts QueryOpts) ([]*things.Task, error) {
	_, tomorrowUnix := currentUTCDayBounds()

	query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
		AND (
			(scheduled_date IS NOT NULL AND scheduled_date < ?)
			OR (today_index_ref IS NOT NULL AND today_index_ref < ?)
		) AND deleted = 0`
	args := []any{tomorrowUnix, tomorrowUnix}
```

- [ ] **Step 2: Update `TasksInAnytime` query (lines 194-202)**

Replace the doc comment and query:

```go
// TasksInAnytime returns tasks in the Anytime view. A task appears in Anytime
// when schedule=1 and it is not classified into Today for the current UTC day.
func (st *State) TasksInAnytime(opts QueryOpts) ([]*things.Task, error) {
	todayUnix, tomorrowUnix := currentUTCDayBounds()

	query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
		AND NOT (
			(scheduled_date IS NOT NULL AND scheduled_date >= ? AND scheduled_date < ?)
			OR (today_index_ref IS NOT NULL AND today_index_ref >= ? AND today_index_ref < ?)
		) AND deleted = 0`
	args := []any{todayUnix, tomorrowUnix, todayUnix, tomorrowUnix}
```

With:

```go
// TasksInAnytime returns tasks in the Anytime view. A task appears in Anytime
// when schedule=1 and it is not classified into Today (no date, or date is in the future).
func (st *State) TasksInAnytime(opts QueryOpts) ([]*things.Task, error) {
	_, tomorrowUnix := currentUTCDayBounds()

	query := `SELECT uuid FROM tasks WHERE type = 0 AND schedule = 1
		AND NOT (
			(scheduled_date IS NOT NULL AND scheduled_date < ?)
			OR (today_index_ref IS NOT NULL AND today_index_ref < ?)
		) AND deleted = 0`
	args := []any{tomorrowUnix, tomorrowUnix}
```

- [ ] **Step 3: Run tests to verify they pass**

Run:
```bash
go test -v -run "TestTasksInTodayIncludesOverdue|TestTasksInAnytimeExcludesOverdue|TestTasksInTodayWithTIR" ./sync/
```

Expected: All three PASS.

- [ ] **Step 4: Run full sync test suite for regressions**

Run:
```bash
go test -v ./sync/...
```

Expected: Exit code 1 (due to pre-existing `TestIntegration/query_change_log` failure). Verify that is the ONLY failing test — all other tests must pass.

- [ ] **Step 5: Commit Piece A**

```bash
git add sync/state.go sync/integration_test.go
git commit -m "Fix TasksInToday to include overdue tasks (schedule=1 with past date)

TasksInToday now matches any date before tomorrow instead of only today's
exact date. TasksInAnytime mirrors this so the two queries remain exact
complements. This matches the real Things 3 app's Today view behavior.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Write failing tests for change detection with overdue tasks

**Files:**
- Modify: `sync/detect_test.go`

These tests exercise `taskLocationAt` and `detectTaskChanges`. They will fail because `isTodayAt` requires an exact calendar-day match.

- [ ] **Step 1: Add `TestTaskLocationOverdue` test cases**

Add these test cases inside `TestTaskLocation` (after the `"anytime schedule uses UTC day boundaries"` test at line 1091):

```go
	t.Run("anytime schedule with past sr date", func(t *testing.T) {
		t.Parallel()
		yesterday := time.Now().AddDate(0, 0, -1)
		task := &things.Task{Schedule: things.TaskScheduleAnytime, ScheduledDate: &yesterday}
		loc := taskLocation(task)
		if loc != LocationToday {
			t.Errorf("expected LocationToday for overdue sr, got %v", loc)
		}
	})

	t.Run("anytime schedule with past tir date", func(t *testing.T) {
		t.Parallel()
		threeDaysAgo := time.Now().AddDate(0, 0, -3)
		task := &things.Task{Schedule: things.TaskScheduleAnytime, TodayIndexReference: &threeDaysAgo}
		loc := taskLocation(task)
		if loc != LocationToday {
			t.Errorf("expected LocationToday for overdue tir, got %v", loc)
		}
	})

	t.Run("anytime schedule with far past sr date", func(t *testing.T) {
		t.Parallel()
		monthAgo := time.Now().AddDate(0, -1, 0)
		task := &things.Task{Schedule: things.TaskScheduleAnytime, ScheduledDate: &monthAgo}
		loc := taskLocation(task)
		if loc != LocationToday {
			t.Errorf("expected LocationToday for overdue sr (30d), got %v", loc)
		}
	})
```

- [ ] **Step 2: Add `TestDetectOverdueTransition` test**

Add this test after `TestTaskLocation` (after line 1131):

```go
func TestDetectOverdueTransition(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("task with past date detected as today", func(t *testing.T) {
		t.Parallel()
		yesterday := time.Now().AddDate(0, 0, -1)
		old := &things.Task{UUID: "t1", Title: "Task", Type: things.TaskTypeTask, Schedule: things.TaskScheduleInbox}
		new := &things.Task{UUID: "t1", Title: "Task", Type: things.TaskTypeTask, Schedule: things.TaskScheduleAnytime, ScheduledDate: &yesterday}
		changes := detectTaskChanges(old, new, 1, now)

		found := false
		for _, c := range changes {
			if _, ok := c.(TaskMovedToToday); ok {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected TaskMovedToToday for overdue task, got different changes")
		}

		for _, c := range changes {
			if _, ok := c.(TaskMovedToAnytime); ok {
				t.Error("overdue task should NOT generate TaskMovedToAnytime")
			}
		}
	})

	t.Run("task with past tir detected as today", func(t *testing.T) {
		t.Parallel()
		threeDaysAgo := time.Now().AddDate(0, 0, -3)
		old := &things.Task{UUID: "t1", Title: "Task", Type: things.TaskTypeTask, Schedule: things.TaskScheduleInbox}
		new := &things.Task{UUID: "t1", Title: "Task", Type: things.TaskTypeTask, Schedule: things.TaskScheduleAnytime, TodayIndexReference: &threeDaysAgo}
		changes := detectTaskChanges(old, new, 1, now)

		found := false
		for _, c := range changes {
			if _, ok := c.(TaskMovedToToday); ok {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected TaskMovedToToday for overdue tir task")
		}
	})
}
```

- [ ] **Step 3: Run new detect tests to verify they fail**

Run:
```bash
go test -v -run "TestTaskLocation/anytime_schedule_with_past|TestDetectOverdueTransition" ./sync/
```

Expected: All three new `TestTaskLocation` subtests FAIL (past dates return `LocationAnytime` instead of `LocationToday`). Both `TestDetectOverdueTransition` subtests FAIL (`TaskMovedToAnytime` generated instead of `TaskMovedToToday`).

---

### Task 4: Implement Piece B — Change detection fix in `sync/detect.go`

**Files:**
- Modify: `sync/detect.go:138-157`

- [ ] **Step 1: Add `isPastOrTodayAt` helper**

Add this function after `isTodayAt` (after line 170):

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

- [ ] **Step 2: Update `taskLocationAt` to use `isPastOrTodayAt`**

In `taskLocationAt` (line 146), replace:

```go
	case things.TaskScheduleAnytime:
		if isTodayAt(t.ScheduledDate, now) || isTodayAt(t.TodayIndexReference, now) {
			return LocationToday
		}
		return LocationAnytime
```

With:

```go
	case things.TaskScheduleAnytime:
		if isPastOrTodayAt(t.ScheduledDate, now) || isPastOrTodayAt(t.TodayIndexReference, now) {
			return LocationToday
		}
		return LocationAnytime
```

- [ ] **Step 3: Run detect tests to verify they pass**

Run:
```bash
go test -v -run "TestTaskLocation|TestDetectOverdueTransition" ./sync/
```

Expected: All PASS, including the new overdue subtests and all existing `TestTaskLocation` cases.

- [ ] **Step 4: Run full test suite for regressions**

Run:
```bash
go test -v ./sync/...
```

Expected: Exit code 1 (due to pre-existing `TestIntegration/query_change_log` failure). Verify that is the ONLY failing test — all other tests must pass.

- [ ] **Step 5: Commit Piece B**

```bash
git add sync/detect.go sync/detect_test.go
git commit -m "Fix change detection to classify overdue tasks as Today

Add isPastOrTodayAt helper and update taskLocationAt to use it for the
TaskScheduleAnytime branch. Tasks with schedule=1 and a past date now
correctly generate TaskMovedToToday instead of TaskMovedToAnytime.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Run full test suite and E2E verification

**Files:** None modified — verification only.

- [ ] **Step 1: Run full project test suite**

Run:
```bash
go test -v ./...
```

Expected: Exit code 1 (due to pre-existing `TestIntegration/query_change_log` failure). Verify that is the ONLY failing test — all other tests must pass.

- [ ] **Step 2: Start local server for E2E testing**

Start the server in the background using `run_in_background` (or `&`):
```bash
export $(grep -v '^#' .env.test | xargs) && go run ./server/ &
```

Wait a few seconds for sync to complete, then verify it's running:
```bash
curl -s http://localhost:9090/ | python3 -c "import sys,json; print(json.loads(sys.stdin.read()))"
```

Expected: `{'service': 'things-cloud-api', 'status': 'ok'}`.

- [ ] **Step 3: Verify overdue task appears in Today**

Run:
```bash
curl -s http://localhost:9090/mcp -X POST -H "Content-Type: application/json" \
  --data-raw '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"things_list_today","arguments":{}}}' | python3 -c "
import sys, json
r = json.loads(sys.stdin.read())
tasks = json.loads(r['result']['content'][0]['text'])
for t in tasks:
    print(f\"  {t['uuid'][:8]}  {t['status']:10}  {t.get('scheduled_date','no-date'):20}  {t['title']}\")
print(f'Total: {len(tasks)} tasks in Today')
"
```

Expected: Any naturally overdue tasks (scheduled before today, still open) appear in the list. The dummy account has a task "Today is here!" scheduled for 2026-04-08, which will be overdue after that date.

- [ ] **Step 4: Verify overdue task is NOT in Anytime**

Run:
```bash
curl -s http://localhost:9090/mcp -X POST -H "Content-Type: application/json" \
  --data-raw '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"things_list_anytime","arguments":{}}}' | python3 -c "
import sys, json
r = json.loads(sys.stdin.read())
tasks = json.loads(r['result']['content'][0]['text'])
for t in tasks:
    print(f\"  {t['uuid'][:8]}  {t.get('scheduled_date','no-date'):20}  {t['title']}\")
print(f'Total: {len(tasks)} tasks in Anytime')
"
```

Expected: No overdue tasks appear. Only tasks with `schedule=1` and no date (true Anytime tasks) are listed.

- [ ] **Step 5: Run smoke tests**

Run:
```bash
./tests/test-smoke.sh http://localhost:9090
```

Expected: 11/11 PASS.

- [ ] **Step 6: Stop the local server**

```bash
lsof -ti:9090 | xargs kill 2>/dev/null; echo "Server stopped"
```
