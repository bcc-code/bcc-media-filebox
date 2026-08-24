-- +goose Up
-- +goose StatementBegin
CREATE TABLE package_access_requests (
    id          TEXT PRIMARY KEY,
    package_id  TEXT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    -- Typed by the requester, deliberately not matched against
    -- package_recipients: a link-only package has no recipient rows.
    email       TEXT NOT NULL,
    -- One kind of ask, "reopen this": what they need goes in 'message', and the
    -- author grants days, downloads, or both from one form regardless.
    message     TEXT NOT NULL DEFAULT '',
    -- Which limit the package had hit when asked, so the author still sees why
    -- after extending it.
    reason      TEXT NOT NULL DEFAULT '' CHECK (reason IN ('', 'expired', 'revoked', 'limit_reached')),
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'granted', 'dismissed')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Serves the author's pending-request list, which orders by created_at.
CREATE INDEX idx_package_access_requests_package ON package_access_requests(package_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- Serves the cooldown lookup. Separate from the index above, which can only seek
-- on package_id here and then scans every row for that package -- and nothing
-- caps how many one package accumulates, since a fresh email always passes.
CREATE INDEX idx_package_access_requests_cooldown ON package_access_requests(package_id, email, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS package_access_requests;
-- +goose StatementEnd
