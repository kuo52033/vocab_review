-- +goose Up
DELETE FROM magic_links;
DELETE FROM sessions;
ALTER TABLE magic_links RENAME COLUMN token TO token_hash;
ALTER TABLE sessions RENAME COLUMN token TO token_hash;
ALTER TABLE magic_links ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE magic_links ADD CONSTRAINT magic_links_email_key UNIQUE (email);

-- +goose Down
DELETE FROM magic_links;
DELETE FROM sessions;
ALTER TABLE magic_links DROP CONSTRAINT IF EXISTS magic_links_email_key;
ALTER TABLE magic_links DROP COLUMN IF EXISTS created_at;
ALTER TABLE magic_links RENAME COLUMN token_hash TO token;
ALTER TABLE sessions RENAME COLUMN token_hash TO token;
