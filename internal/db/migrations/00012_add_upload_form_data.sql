-- +goose Up
-- +goose StatementBegin
ALTER TABLE uploads ADD COLUMN form_data TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE uploads DROP COLUMN form_data;
-- +goose StatementEnd
