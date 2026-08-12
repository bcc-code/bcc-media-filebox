-- +goose Up
-- +goose StatementBegin
ALTER TABLE shares ADD COLUMN max_access_count INTEGER;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE shares DROP COLUMN max_access_count;
-- +goose StatementEnd
