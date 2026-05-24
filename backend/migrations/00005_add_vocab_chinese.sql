-- +goose Up
ALTER TABLE vocab_items
    ADD COLUMN chinese TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE vocab_items
    DROP COLUMN chinese;
