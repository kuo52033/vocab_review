-- +goose Up
ALTER TABLE vocab_items
    DROP COLUMN IF EXISTS kind;

-- +goose Down
ALTER TABLE vocab_items
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'word' CHECK (kind IN ('word', 'phrase'));
