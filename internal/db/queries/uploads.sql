-- name: CreateUpload :exec
INSERT INTO uploads (id, user_id, filename, size, content_type, is_partial, final_upload_id, sha256)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUpload :one
SELECT * FROM uploads WHERE id = ?;

-- name: UpdateUploadOffset :exec
UPDATE uploads SET offset = ? WHERE id = ?;

-- name: CompleteUpload :exec
UPDATE uploads
SET status = 'completed',
    offset = size,
    duration_ms = CAST((julianday(CURRENT_TIMESTAMP) - julianday(created_at)) * 86400000 AS INTEGER),
    completed_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateDurationMs :exec
UPDATE uploads SET duration_ms = ? WHERE id = ?;

-- name: FailUpload :exec
UPDATE uploads SET status = 'failed' WHERE id = ?;

-- name: ListUploads :many
SELECT * FROM uploads WHERE is_partial = 0 AND status = 'completed' AND user_id = ? ORDER BY created_at DESC;

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = ?;

-- name: DeletePartialUploads :exec
DELETE FROM uploads WHERE final_upload_id = ?;
