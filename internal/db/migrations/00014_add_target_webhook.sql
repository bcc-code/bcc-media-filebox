-- +goose Up
-- +goose StatementBegin
ALTER TABLE targets ADD COLUMN webhook_url TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE targets DROP COLUMN webhook_url;
-- +goose StatementEnd
