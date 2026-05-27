-- +goose Up
ALTER TABLE magic_links ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'magic_links_email_key'
          AND conrelid = 'magic_links'::regclass
    ) THEN
        ALTER TABLE magic_links ADD CONSTRAINT magic_links_email_key UNIQUE (email);
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE magic_links DROP CONSTRAINT IF EXISTS magic_links_email_key;
ALTER TABLE magic_links DROP COLUMN IF EXISTS created_at;
