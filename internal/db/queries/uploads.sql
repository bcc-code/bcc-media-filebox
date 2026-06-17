-- name: CreateUpload :exec
INSERT INTO uploads (id, user_id, filename, size, content_type, is_partial, final_upload_id, sha256, target_name, form_data)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ProjectSeasons :many
SELECT DISTINCT CAST(json_extract(form_data, '$.season') AS TEXT) AS value
FROM uploads
WHERE status = 'completed' AND form_data IS NOT NULL
  AND CAST(json_extract(form_data, '$.project') AS TEXT) = ?
  AND COALESCE(CAST(json_extract(form_data, '$.season') AS TEXT), '') <> ''
ORDER BY value;

-- name: ProjectEpisodes :many
SELECT DISTINCT CAST(json_extract(form_data, '$.episode') AS TEXT) AS value
FROM uploads
WHERE status = 'completed' AND form_data IS NOT NULL
  AND CAST(json_extract(form_data, '$.project') AS TEXT) = ?
  AND COALESCE(CAST(json_extract(form_data, '$.episode') AS TEXT), '') <> ''
ORDER BY value;

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

-- name: UpdateUploadFilename :exec
UPDATE uploads SET filename = ? WHERE id = ?;

-- name: ListUploads :many
SELECT * FROM uploads WHERE is_partial = 0 AND status = 'completed' AND user_id = ? ORDER BY created_at DESC;

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = ?;

-- name: DeletePartialUploads :exec
DELETE FROM uploads WHERE final_upload_id = ?;
