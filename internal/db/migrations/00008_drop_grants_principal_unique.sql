-- +goose Up
-- +goose StatementBegin
CREATE TABLE grants_new (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    principal_kind  TEXT NOT NULL CHECK (principal_kind IN ('user', 'group')),
    principal_value TEXT NOT NULL,
    admin           INTEGER NOT NULL DEFAULT 0,
    all_targets     INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO grants_new (id, principal_kind, principal_value, admin, all_targets, created_at)
SELECT id, principal_kind, principal_value, admin, all_targets, created_at FROM grants;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE grants;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE grants_new RENAME TO grants;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_grants_principal ON grants(principal_kind, principal_value);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_grants_principal;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE grants_old (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    principal_kind  TEXT NOT NULL CHECK (principal_kind IN ('user', 'group')),
    principal_value TEXT NOT NULL,
    admin           INTEGER NOT NULL DEFAULT 0,
    all_targets     INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (principal_kind, principal_value)
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO grants_old (id, principal_kind, principal_value, admin, all_targets, created_at)
SELECT MIN(id), principal_kind, principal_value, MAX(admin), MAX(all_targets), MIN(created_at)
FROM grants
GROUP BY principal_kind, principal_value;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE grants;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE grants_old RENAME TO grants;
-- +goose StatementEnd
