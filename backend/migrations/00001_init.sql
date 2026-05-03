-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE magic_links (
    token TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE vocab_items (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    term TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('word', 'phrase')),
    meaning TEXT NOT NULL,
    example_sentence TEXT NOT NULL,
    source_text TEXT NOT NULL,
    source_url TEXT NOT NULL,
    notes TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ
);

CREATE TABLE review_states (
    vocab_item_id TEXT PRIMARY KEY REFERENCES vocab_items (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('new', 'learning', 'review')),
    ease_factor DOUBLE PRECISION NOT NULL,
    interval_days INTEGER NOT NULL,
    repetition_count INTEGER NOT NULL,
    last_reviewed_at TIMESTAMPTZ,
    next_due_at TIMESTAMPTZ NOT NULL,
    consecutive_again INTEGER NOT NULL
);

CREATE TABLE review_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vocab_item_id TEXT NOT NULL REFERENCES vocab_items (id) ON DELETE CASCADE,
    grade TEXT NOT NULL CHECK (grade IN ('again', 'hard', 'good', 'easy')),
    reviewed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE capture_sources (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vocab_item_id TEXT NOT NULL REFERENCES vocab_items (id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    selection TEXT NOT NULL,
    page_title TEXT NOT NULL,
    page_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE device_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (user_id, token)
);

CREATE TABLE notification_jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vocab_item_id TEXT NOT NULL REFERENCES vocab_items (id) ON DELETE CASCADE,
    scheduled_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    status TEXT NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
    message TEXT NOT NULL
);

CREATE UNIQUE INDEX notification_jobs_pending_unique
    ON notification_jobs (user_id, vocab_item_id)
    WHERE status = 'pending';

CREATE INDEX review_states_user_due_idx ON review_states (user_id, next_due_at);
CREATE INDEX vocab_items_user_created_idx ON vocab_items (user_id, created_at);
CREATE INDEX notification_jobs_user_scheduled_idx ON notification_jobs (user_id, scheduled_at);

-- +goose Down
DROP INDEX notification_jobs_user_scheduled_idx;
DROP INDEX vocab_items_user_created_idx;
DROP INDEX review_states_user_due_idx;
DROP INDEX notification_jobs_pending_unique;
DROP TABLE notification_jobs;
DROP TABLE device_tokens;
DROP TABLE capture_sources;
DROP TABLE review_logs;
DROP TABLE review_states;
DROP TABLE vocab_items;
DROP TABLE magic_links;
DROP TABLE sessions;
DROP TABLE users;
