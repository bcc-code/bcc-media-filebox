-- name: CreateShare :one
INSERT INTO shares (id, package_id, upload_id)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetShareByID :one
SELECT * FROM shares WHERE id = ?;

-- name: IncrementShareAccessCount :one
UPDATE shares
SET access_count = access_count + 1
WHERE id = ?
RETURNING *;

-- name: IncrementShareAccessCountIfUnderLimit :one
UPDATE shares
SET access_count = access_count + 1
WHERE id = ? AND access_count < ?
RETURNING *;
