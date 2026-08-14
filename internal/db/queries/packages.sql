-- name: CreatePackage :one
INSERT INTO packages (id, created_by_user_id, name, message, verification_method, password_hash, expires_at, max_downloads)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPackageByID :one
SELECT * FROM packages WHERE id = ?;

-- name: ListPackagesByUser :many
SELECT * FROM packages
WHERE created_by_user_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountPackagesByUser :one
SELECT COUNT(*) FROM packages WHERE created_by_user_id = ?;

-- name: RevokePackage :exec
UPDATE packages SET status = 'revoked' WHERE id = ?;

-- name: IncrementPackageDownloadCount :one
UPDATE packages
SET download_count = download_count + 1
WHERE id = ?
RETURNING *;

-- name: ListSharesByPackageID :many
SELECT s.id AS share_id, s.upload_id, u.filename, u.size, s.access_count
FROM shares s
JOIN uploads u ON u.id = s.upload_id
WHERE s.package_id = ?
ORDER BY s.created_at ASC;

-- name: GetPackageMaxAccessCount :one
-- max_downloads is a per-file budget (see GetShare), so the download count
-- shown to the package owner should reflect whichever file has been
-- downloaded the most, not the sum of downloads across every file.
SELECT CAST(COALESCE(MAX(access_count), 0) AS INTEGER) FROM shares WHERE package_id = ?;

-- name: CreatePackageRecipient :one
INSERT INTO package_recipients (package_id, email)
VALUES (?, ?)
RETURNING *;

-- name: ListPackageRecipientsByPackageID :many
SELECT * FROM package_recipients WHERE package_id = ? ORDER BY id ASC;

-- name: GetPackageRecipientByEmail :one
SELECT * FROM package_recipients WHERE package_id = ? AND email = ?;

-- name: GetPackageRecipientByMagicLinkToken :one
SELECT * FROM package_recipients WHERE magic_link_token = ?;

-- name: UpdatePackageRecipientOTP :exec
UPDATE package_recipients
SET otp_code_hash = ?, otp_expires_at = ?
WHERE id = ?;

-- name: SetPackageRecipientMagicLinkToken :exec
UPDATE package_recipients
SET magic_link_token = ?
WHERE id = ?;

-- name: MarkPackageRecipientVerified :exec
UPDATE package_recipients
SET verified_at = CURRENT_TIMESTAMP
WHERE id = ?;
