-- +goose Up
ALTER TABLE uploads ADD COLUMN sha256 TEXT;

-- +goose Down
ALTER TABLE uploads DROP COLUMN sha256;
