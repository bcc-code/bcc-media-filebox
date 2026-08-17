-- +goose Up
-- +goose StatementBegin
ALTER TABLE package_recipients ADD COLUMN sent_at DATETIME;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE package_recipients ADD COLUMN send_error TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE package_recipients DROP COLUMN send_error;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE package_recipients DROP COLUMN sent_at;
-- +goose StatementEnd
