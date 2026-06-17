-- +goose Up
-- +goose StatementBegin
ALTER TABLE targets ADD COLUMN form_key TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE targets DROP COLUMN form_key;
-- +goose StatementEnd
