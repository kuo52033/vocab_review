-- +goose Up
ALTER TABLE notification_jobs
    DROP CONSTRAINT notification_jobs_status_check;

ALTER TABLE notification_jobs
    ADD CONSTRAINT notification_jobs_status_check CHECK (status IN ('pending', 'processing', 'sent', 'failed'));

DROP INDEX notification_jobs_pending_unique;

CREATE UNIQUE INDEX notification_jobs_pending_unique
    ON notification_jobs (user_id, vocab_item_id)
    WHERE status IN ('pending', 'processing');

-- +goose Down
UPDATE notification_jobs
SET status = 'pending'
WHERE status = 'processing';

DROP INDEX notification_jobs_pending_unique;

CREATE UNIQUE INDEX notification_jobs_pending_unique
    ON notification_jobs (user_id, vocab_item_id)
    WHERE status = 'pending';

ALTER TABLE notification_jobs
    DROP CONSTRAINT notification_jobs_status_check;

ALTER TABLE notification_jobs
    ADD CONSTRAINT notification_jobs_status_check CHECK (status IN ('pending', 'sent', 'failed'));
