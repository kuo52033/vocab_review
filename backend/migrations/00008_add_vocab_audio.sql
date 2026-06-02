-- +goose Up
CREATE TABLE vocab_audios (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    voice TEXT NOT NULL,
    speed NUMERIC(4,2) NOT NULL,
    output_format TEXT NOT NULL,
    input_text TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    storage_provider TEXT NOT NULL DEFAULT 's3',
    storage_bucket TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'audio/mpeg',
    file_size_bytes BIGINT,
    duration_ms INTEGER,
    status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready')),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (provider, model, voice, speed, output_format, input_hash)
);

ALTER TABLE vocab_items
    ADD COLUMN audio_id TEXT REFERENCES vocab_audios (id);

CREATE TABLE vocab_audio_jobs (
    id TEXT PRIMARY KEY,
    vocab_item_id TEXT NOT NULL UNIQUE REFERENCES vocab_items (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    voice TEXT NOT NULL,
    speed NUMERIC(4,2) NOT NULL,
    output_format TEXT NOT NULL,
    input_text TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_error TEXT,
    audio_id TEXT REFERENCES vocab_audios (id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX vocab_audio_jobs_claim_idx
    ON vocab_audio_jobs (status, next_attempt_at, created_at);

CREATE INDEX vocab_audio_jobs_hash_idx
    ON vocab_audio_jobs (provider, model, voice, speed, output_format, input_hash);

-- +goose Down
DROP INDEX vocab_audio_jobs_hash_idx;
DROP INDEX vocab_audio_jobs_claim_idx;
DROP TABLE vocab_audio_jobs;
ALTER TABLE vocab_items DROP COLUMN audio_id;
DROP TABLE vocab_audios;
