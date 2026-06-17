-- +goose Up
-- +goose StatementBegin
ALTER TABLE targets ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE targets SET position = id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE targets DROP COLUMN position;
-- +goose StatementEnd
