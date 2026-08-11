-- name: CreateShare :one
INSERT INTO shares (id, created_by_user_id, upload_id, expires_at, requires_auth)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetShareByID :one
SELECT * FROM shares WHERE id = ?;

-- name: GetSharesByUploadID :many
SELECT * FROM shares WHERE upload_id = ?;

-- name: GetSharesByUserID :many
SELECT * FROM shares WHERE created_by_user_id = ?;

-- name: ListSharesByUserPaginated :many
SELECT s.id AS share_id, s.upload_id, u.filename, s.created_at, s.expires_at, s.access_count
FROM shares s
JOIN uploads u ON u.id = s.upload_id
WHERE s.created_by_user_id = ?
ORDER BY s.created_at DESC
LIMIT ? OFFSET ?;

-- name: CountSharesByUserID :one
SELECT COUNT(*) FROM shares WHERE created_by_user_id = ?;

-- name: UpdateShareAccessCount :one
UPDATE shares
SET access_count = access_count + 1
WHERE id = ?
RETURNING *;

-- name: DeleteShare :exec
DELETE FROM shares WHERE id = ?;

-- name: DeleteExpiredShares :exec
DELETE FROM shares WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP;

-- name: GetActiveShareByID :one
SELECT * FROM shares
WHERE id = ?
  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP);
