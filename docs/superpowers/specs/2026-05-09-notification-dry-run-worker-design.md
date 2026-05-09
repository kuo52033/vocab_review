# Notification Dry-Run Worker Design

## Goal

Add the first backend notification delivery path without APNs credentials. The worker should prove the complete local lifecycle: find due notification jobs, claim them safely, dry-run deliver to registered devices, and mark delivered jobs as sent.

## Scope

This pass implements a separate backend worker command and a dry-run sender only. It does not send real Apple push notifications, add APNs credentials, or change the iOS notification permission flow. If no device token exists for a job's user, the job stays pending.

## Command Shape

Add `backend/cmd/notifications` as a separate executable from the API server.

Local commands:

```bash
make notifications-run
cd backend && go run ./cmd/notifications -once
```

Default mode runs continuously with a `10s` poll interval. The `-once` flag processes one batch and exits for local verification and automated tests.

## Backend Components

Create `backend/internal/service/notifications` with:

- `Worker`: owns polling, batch processing, and logging.
- `Sender`: interface for delivery implementations.
- `DryRunSender`: logs the notification payload instead of calling APNs.

The first sender mode is always dry-run. Future APNs support should plug in behind the same `Sender` interface.

## Repository Operations

Add domain-specific repository methods:

```go
ClaimDueNotificationJobs(ctx context.Context, now time.Time, limit int) ([]domain.NotificationJob, error)
ListDeviceTokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error)
MarkNotificationSent(ctx context.Context, jobID string, sentAt time.Time) error
MarkNotificationFailed(ctx context.Context, jobID string) error
```

Postgres claiming should use a transaction and `FOR UPDATE SKIP LOCKED` so multiple workers cannot process the same job concurrently. Claiming only returns jobs with `status = 'pending'` and `scheduled_at <= now`.

## Worker Behavior

For each claimed job:

1. Load device tokens for `job.user_id`.
2. If there are no tokens, log clearly and leave the job pending.
3. If tokens exist, call `DryRunSender` for each token.
4. If at least one dry-run send succeeds, mark the job `sent`.
5. If all sends fail, mark the job `failed`.

Dry-run logs should include job ID, user ID, vocab item ID, device count, scheduled time, and message.

## Testing

Add unit tests with hand-written fakes for:

- Due jobs with tokens are dry-run delivered and marked sent.
- Due jobs without tokens stay pending.
- Sender failure marks the job failed when all token sends fail.
- `-once` mode processes one batch and exits.

Add Postgres integration coverage for:

- Claiming only due pending jobs.
- Claimed jobs are safe under transaction locking.
- Marking jobs sent updates `status` and `sent_at`.

## MVP Fit

This completes the backend delivery lifecycle locally while avoiding APNs setup risk. The next feature after this should be real APNs delivery plus iOS device-token registration verification.
