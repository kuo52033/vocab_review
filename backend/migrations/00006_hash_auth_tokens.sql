-- +goose Up
DELETE FROM magic_links;
DELETE FROM sessions;
ALTER TABLE magic_links RENAME COLUMN token TO token_hash;
ALTER TABLE sessions RENAME COLUMN token TO token_hash;

-- +goose Down
DELETE FROM magic_links;
DELETE FROM sessions;
ALTER TABLE magic_links RENAME COLUMN token_hash TO token;
ALTER TABLE sessions RENAME COLUMN token_hash TO token;
