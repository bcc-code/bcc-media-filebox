-- +goose Up
-- +goose StatementBegin
CREATE TABLE packages (
    id                   TEXT PRIMARY KEY,
    created_by_user_id   INTEGER NOT NULL REFERENCES users(id),
    name                 TEXT NOT NULL,
    message              TEXT NOT NULL DEFAULT '',
    verification_method  TEXT NOT NULL DEFAULT 'none' CHECK (verification_method IN ('none', 'email_otp', 'magic_link', 'password', 'bcc_login')),
    password_hash        TEXT,
    expires_at           DATETIME NOT NULL,
    max_downloads        INTEGER,
    download_count       INTEGER NOT NULL DEFAULT 0,
    notify_on_download   INTEGER NOT NULL DEFAULT 0,
    status               TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE shares (
    id          TEXT PRIMARY KEY,
    package_id  TEXT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    upload_id   TEXT NOT NULL REFERENCES uploads(id),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE package_recipients (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    package_id       TEXT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    email            TEXT NOT NULL,
    otp_code_hash    TEXT,
    otp_expires_at   DATETIME,
    magic_link_token TEXT,
    verified_at      DATETIME,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS package_recipients;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS shares;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS packages;
-- +goose StatementEnd
