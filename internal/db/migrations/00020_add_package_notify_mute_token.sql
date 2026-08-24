-- +goose Up
-- The capability behind the "stop these notifications" link in the download
-- report: the author's mail client need not be signed in, so the token in the
-- link is the only credential. Backfilled for packages that predate it, since a
-- mail can still go out for any package that is still active.
ALTER TABLE packages ADD COLUMN notify_mute_token TEXT;

UPDATE packages SET notify_mute_token = lower(hex(randomblob(16)))
WHERE notify_mute_token IS NULL;

CREATE UNIQUE INDEX idx_packages_notify_mute_token
    ON packages(notify_mute_token) WHERE notify_mute_token IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_packages_notify_mute_token;
ALTER TABLE packages DROP COLUMN notify_mute_token;
