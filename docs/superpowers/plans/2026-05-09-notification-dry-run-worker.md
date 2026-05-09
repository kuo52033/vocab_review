# Notification Dry-Run Worker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a separate backend notification worker that dry-run delivers due notification jobs and marks delivered jobs as sent.

**Architecture:** Add focused notification repository operations, then build `backend/internal/service/notifications` around a `Worker` and `Sender` interface. Add `backend/cmd/notifications` as a separate executable using the same Postgres startup pattern as the API server.

**Tech Stack:** Go standard library, pgx/Postgres, slog, existing Makefile workflow.

---

## File Structure

- Modify `backend/internal/repository/contracts.go`: add notification worker repository methods.
- Create `backend/migrations/00003_add_processing_notification_status.sql`: add `processing` status for safe claims.
- Modify `backend/internal/repository/postgres/device.go`: add `ListDeviceTokensForUser`.
- Modify `backend/internal/repository/postgres/notification.go`: add claim, mark pending, mark sent, and mark failed operations.
- Modify `backend/internal/repository/postgres/store_integration_test.go`: cover due-job claiming and sent marking.
- Create `backend/internal/service/notifications/worker.go`: worker, dry-run sender, repository interface, config.
- Create `backend/internal/service/notifications/worker_test.go`: worker unit tests with fakes.
- Create `backend/cmd/notifications/main.go`: command entry point with `-once`, `-interval`, and `-batch-size`.
- Modify `Makefile`: add `notifications-run`.

## Task 1: Repository Contract And Postgres Operations

**Files:**
- Modify: `backend/internal/repository/contracts.go`
- Modify: `backend/internal/repository/postgres/device.go`
- Modify: `backend/internal/repository/postgres/notification.go`
- Modify: `backend/internal/repository/postgres/store_integration_test.go`

- [ ] **Step 1: Add failing integration coverage**

Add a test that creates one due pending job, one future pending job, and one sent job. Verify `ClaimDueNotificationJobs(ctx, now, 10)` returns only the due pending job. Verify `ListDeviceTokensForUser` returns registered tokens and `MarkNotificationSent` updates `status` plus `sent_at`.

- [ ] **Step 2: Run the failing repository test**

```bash
make test-integration
```

Expected: fail with missing repository methods.

- [ ] **Step 3: Add repository method signatures**

Add to `NotificationRepository`:

```go
ClaimDueNotificationJobs(ctx context.Context, now time.Time, limit int) ([]domain.NotificationJob, error)
ListDeviceTokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error)
MarkNotificationPending(ctx context.Context, jobID string) error
MarkNotificationSent(ctx context.Context, jobID string, sentAt time.Time) error
MarkNotificationFailed(ctx context.Context, jobID string) error
```

- [ ] **Step 4: Implement Postgres methods**

Use `FOR UPDATE SKIP LOCKED` inside `ClaimDueNotificationJobs`, then update claimed rows to `processing` before returning:

```sql
WITH claimed AS (
    SELECT id
    FROM notification_jobs
    WHERE status = 'pending' AND scheduled_at <= $1
    ORDER BY scheduled_at ASC
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
UPDATE notification_jobs AS jobs
SET status = 'processing'
FROM claimed
WHERE jobs.id = claimed.id
RETURNING jobs.id, jobs.user_id, jobs.vocab_item_id, jobs.scheduled_at, jobs.sent_at, jobs.status, jobs.message
```

Use `UPDATE notification_jobs SET status = 'pending' WHERE id = $1` when a claimed job has no devices, `UPDATE notification_jobs SET status = 'sent', sent_at = $2 WHERE id = $1` for sent, and `UPDATE notification_jobs SET status = 'failed' WHERE id = $1` for failed.

- [ ] **Step 5: Verify repository tests**

```bash
make test-integration
```

Expected: pass.

## Task 2: Notification Worker Service

**Files:**
- Create: `backend/internal/service/notifications/worker.go`
- Create: `backend/internal/service/notifications/worker_test.go`

- [ ] **Step 1: Add failing worker unit tests**

Cover:

- due job with one token calls sender and marks sent.
- due job without tokens is not marked sent or failed.
- all sender failures mark the job failed.
- `RunOnce` processes one batch and exits.

- [ ] **Step 2: Run failing worker tests**

```bash
cd backend && go test ./internal/service/notifications
```

Expected: fail because package does not exist.

- [ ] **Step 3: Implement worker types**

Create:

```go
type Repository interface {
    ClaimDueNotificationJobs(ctx context.Context, now time.Time, limit int) ([]domain.NotificationJob, error)
    ListDeviceTokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error)
    MarkNotificationPending(ctx context.Context, jobID string) error
    MarkNotificationSent(ctx context.Context, jobID string, sentAt time.Time) error
    MarkNotificationFailed(ctx context.Context, jobID string) error
}

type Sender interface {
    Send(ctx context.Context, token domain.DeviceToken, job domain.NotificationJob) error
}

type Worker struct {
    repo Repository
    sender Sender
    clock clock.Clock
    logger *slog.Logger
    batchSize int
}
```

Expose `RunOnce(ctx context.Context) error` and `Run(ctx context.Context, interval time.Duration) error`.

- [ ] **Step 4: Implement dry-run sender**

`DryRunSender` logs `notification_dry_run` with job ID, user ID, vocab item ID, device token ID, platform, scheduled time, and message, then returns nil.

- [ ] **Step 5: Verify worker tests**

```bash
cd backend && go test ./internal/service/notifications
```

Expected: pass.

## Task 3: Worker Command And Local Workflow

**Files:**
- Create: `backend/cmd/notifications/main.go`
- Modify: `Makefile`

- [ ] **Step 1: Implement command**

Use flags:

```text
-once
-interval=10s
-batch-size=50
```

Read `DATABASE_URL`, connect to Postgres, create a dry-run sender, and run the worker. Fail fast with a clear message if `DATABASE_URL` is missing.

- [ ] **Step 2: Add Makefile command**

Add:

```make
notifications-run:
	test -n "$(DATABASE_URL)"
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/notifications
```

- [ ] **Step 3: Verify full backend test suite**

```bash
cd backend && go test ./...
```

Expected: pass.

- [ ] **Step 4: Verify one-shot command starts**

```bash
cd backend && DATABASE_URL="postgres://vocab:vocab@127.0.0.1:5432/vocab_review_dev?sslmode=disable" go run ./cmd/notifications -once
```

Expected: command exits successfully after logging one batch result.

## Self-Review

- Spec coverage: repository operations, worker behavior, dry-run sender, separate command, loop and `-once`, no-token pending behavior, and tests are covered.
- Placeholder scan: no TODO/TBD placeholders.
- Type consistency: repository and worker method names match the design spec.
