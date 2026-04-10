-- name: CreateUpload :exec
INSERT INTO uploads (id, filename, size, content_type, is_partial, final_upload_id)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetUpload :one
SELECT * FROM uploads WHERE id = ?;

-- name: UpdateUploadOffset :exec
UPDATE uploads SET offset = ? WHERE id = ?;

-- name: CompleteUpload :exec
UPDATE uploads SET status = 'completed', offset = size, completed_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: FailUpload :exec
UPDATE uploads SET status = 'failed' WHERE id = ?;

-- name: ListUploads :many
SELECT * FROM uploads WHERE is_partial = 0 ORDER BY created_at DESC;

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = ?;

-- name: DeletePartialUploads :exec
DELETE FROM uploads WHERE final_upload_id = ?;
