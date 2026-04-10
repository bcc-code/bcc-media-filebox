-- +goose Up
ALTER TABLE uploads ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE uploads ADD COLUMN duration_ms INTEGER;

-- +goose Down
ALTER TABLE uploads DROP COLUMN user_id;
ALTER TABLE uploads DROP COLUMN duration_ms;
