-- name: CreatePackage :one
INSERT INTO packages (id, created_by_user_id, name, message, verification_method, password_hash, expires_at, max_downloads, notify_on_download, notify_mute_token)
VALUES (@id, @created_by_user_id, @name, @message, @verification_method, @password_hash, @expires_at, @max_downloads, @notify_on_download, @notify_mute_token)
RETURNING *;

-- name: GetPackageByID :one
SELECT * FROM packages WHERE id = @id;

-- name: ListPackagesByUser :many
SELECT * FROM packages
WHERE created_by_user_id = @created_by_user_id
ORDER BY created_at DESC
LIMIT @limit OFFSET @offset;

-- name: CountPackagesByUser :one
SELECT COUNT(*) FROM packages WHERE created_by_user_id = @created_by_user_id;

-- name: RevokePackage :exec
UPDATE packages SET status = 'revoked' WHERE id = @id;

-- name: SetPackageNotifyOnDownload :exec
UPDATE packages SET notify_on_download = @notify_on_download WHERE id = @id;

-- name: MutePackageNotificationsByToken :one
-- The mail's one-click opt-out. Idempotent: an already-muted package still
-- matches, so a second click confirms rather than 404s. Returns the row so the
-- page can name the package it just quietened.
UPDATE packages SET notify_on_download = 0
WHERE notify_mute_token = @notify_mute_token AND notify_mute_token IS NOT NULL
RETURNING *;

-- name: IncrementPackageDownloadCount :one
UPDATE packages
SET download_count = download_count + 1
WHERE id = @id
RETURNING *;

-- name: ListSharesByPackageID :many
SELECT s.id AS share_id, s.upload_id, u.filename, u.size, s.access_count
FROM shares s
JOIN uploads u ON u.id = s.upload_id
WHERE s.package_id = @package_id
ORDER BY s.created_at ASC;

-- name: GetPackageMaxAccessCount :one
-- Legacy/fallback aggregate. New package artifacts keep member share counters
-- in sync, so this still reads as the most-downloaded artifact rather than a
-- sum across the package.
SELECT CAST(COALESCE(MAX(access_count), 0) AS INTEGER) FROM shares WHERE package_id = @package_id;

-- name: GetPackageMinAccessCount :one
-- Counterpart to GetPackageMaxAccessCount: once even the least-downloaded share
-- has hit max_downloads, every file has, so nothing is left to download.
SELECT CAST(COALESCE(MIN(access_count), 0) AS INTEGER) FROM shares WHERE package_id = @package_id;

-- name: ListSharesByPackageIDs :many
-- Batch form, for the author's package list.
SELECT s.package_id, s.id AS share_id, s.upload_id, u.filename, u.size, s.access_count
FROM shares s
JOIN uploads u ON u.id = s.upload_id
WHERE s.package_id IN (sqlc.slice('package_ids'))
ORDER BY s.package_id, s.created_at ASC;

-- name: GetPackageAccessCountsByPackageIDs :many
-- Not derived from ListSharesByPackageIDs: that joins uploads, and a share whose
-- upload was deleted still counts here, as in the single-package queries.
SELECT package_id,
       CAST(COALESCE(MAX(access_count), 0) AS INTEGER) AS max_access_count,
       CAST(COALESCE(MIN(access_count), 0) AS INTEGER) AS min_access_count
FROM shares
WHERE package_id IN (sqlc.slice('package_ids'))
GROUP BY package_id;

-- name: CreatePackageRecipient :one
INSERT INTO package_recipients (package_id, email)
VALUES (@package_id, @email)
RETURNING *;

-- name: ListPackageRecipientsByPackageID :many
SELECT * FROM package_recipients WHERE package_id = @package_id ORDER BY id ASC;

-- name: ListPackageRecipientsByPackageIDs :many
SELECT * FROM package_recipients
WHERE package_id IN (sqlc.slice('package_ids'))
ORDER BY package_id, id ASC;

-- name: GetPackageRecipientByEmail :one
SELECT * FROM package_recipients WHERE package_id = @package_id AND email = @email;

-- name: GetPackageRecipientByMagicLinkToken :one
SELECT * FROM package_recipients WHERE magic_link_token = @magic_link_token;

-- name: UpdatePackageRecipientOTP :exec
UPDATE package_recipients
SET otp_code_hash = @otp_code_hash, otp_expires_at = @otp_expires_at
WHERE id = @id;

-- name: SetPackageRecipientMagicLinkToken :exec
UPDATE package_recipients
SET magic_link_token = @magic_link_token
WHERE id = @id;

-- name: MarkPackageRecipientVerified :exec
UPDATE package_recipients
SET verified_at = CURRENT_TIMESTAMP
WHERE id = @id;

-- name: MarkPackageRecipientSent :exec
UPDATE package_recipients
SET sent_at = CURRENT_TIMESTAMP, send_error = NULL
WHERE id = @id;

-- name: MarkPackageRecipientSendFailed :exec
UPDATE package_recipients
SET sent_at = NULL, send_error = @send_error
WHERE id = @id;

-- name: ExtendPackage :one
-- Pushes expiry out, replaces the per-artifact download budget, and clears
-- 'revoked': an author who explicitly extends means to make it reachable.
UPDATE packages
SET expires_at    = @expires_at,
    max_downloads = @max_downloads,
    status        = 'active'
WHERE id = @id
RETURNING *;

-- name: CreatePackageAccessRequest :one
INSERT INTO package_access_requests (id, package_id, email, message, reason)
VALUES (@id, @package_id, @email, @message, @reason)
RETURNING *;

-- name: GetLatestPackageAccessRequestByEmail :one
-- Backs the per-requester cooldown. Ordered by created_at, since ids are random
-- tokens rather than a sequence.
SELECT * FROM package_access_requests
WHERE package_id = @package_id AND email = @email
ORDER BY created_at DESC
LIMIT 1;

-- name: ListPendingPackageAccessRequests :many
SELECT * FROM package_access_requests
WHERE package_id = @package_id AND status = 'pending'
ORDER BY created_at DESC;

-- name: ListPendingPackageAccessRequestsByPackageIDs :many
SELECT * FROM package_access_requests
WHERE package_id IN (sqlc.slice('package_ids')) AND status = 'pending'
ORDER BY package_id, created_at DESC;

-- name: GetPackageAccessRequest :one
SELECT * FROM package_access_requests WHERE id = @id;

-- name: DismissPackageAccessRequest :exec
UPDATE package_access_requests
SET status = 'dismissed', resolved_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'pending';

-- name: GrantPendingPackageAccessRequests :many
-- One extend answers every outstanding request. Returns the rows it resolved so
-- the caller can mail those requesters.
UPDATE package_access_requests
SET status = 'granted', resolved_at = CURRENT_TIMESTAMP
WHERE package_id = @package_id AND status = 'pending'
RETURNING *;

-- name: CountRecentPackageAccessRequests :one
-- The per-package ceiling. Compared in seconds rather than by binding a Go time,
-- since created_at is written by CURRENT_TIMESTAMP and the two formats differ.
SELECT COUNT(*) FROM package_access_requests
WHERE package_id = @package_id
  AND unixepoch(created_at) > unixepoch('now') - CAST(@window_seconds AS INTEGER);
