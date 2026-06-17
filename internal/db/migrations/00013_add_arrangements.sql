-- +goose Up
-- +goose StatementBegin
CREATE TABLE arrangements (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    code       TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE sub_events (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    arrangement_id INTEGER NOT NULL REFERENCES arrangements(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    code           TEXT NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(arrangement_id, name),
    UNIQUE(arrangement_id, code)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_sub_events_arrangement ON sub_events(arrangement_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE sub_events;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE arrangements;
-- +goose StatementEnd
