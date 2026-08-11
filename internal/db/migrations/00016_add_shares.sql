-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS shares (
    id TEXT PRIMARY KEY,
    created_by_user_id INTEGER NOT NULL,
    upload_id TEXT NOT NULL,
    expires_at DATETIME,
    access_count INTEGER NOT NULL DEFAULT 0,
    requires_auth TEXT NOT NULL DEFAULT 'none' CHECK (requires_auth IN ('none', 'bcc')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id),
    FOREIGN KEY (upload_id) REFERENCES uploads(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS shares;
-- +goose StatementEnd
