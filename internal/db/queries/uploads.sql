-- name: CreateUpload :exec
INSERT INTO uploads (id, user_id, filename, size, content_type, is_partial, final_upload_id, sha256, target_name, form_data)
VALUES (@id, @user_id, @filename, @size, @content_type, @is_partial, @final_upload_id, @sha256, @target_name, @form_data);

-- name: CreatePendingUpload :exec
-- TUS completion only means all bytes arrived in the temporary area. The
-- asynchronous finalizer marks this ready after the final move/S3 upload.
INSERT INTO uploads (
    id, user_id, filename, size, content_type, is_partial, final_upload_id,
    sha256, target_name, form_data, storage_status
)
VALUES (@id, @user_id, @filename, @size, @content_type, @is_partial, @final_upload_id, @sha256, @target_name, @form_data, 'pending');

-- name: ProjectSeasons :many
SELECT DISTINCT CAST(json_extract(form_data, '$.season') AS TEXT) AS value
FROM uploads
WHERE status = 'completed' AND form_data IS NOT NULL
  AND CAST(json_extract(form_data, '$.project') AS TEXT) = @project
  AND COALESCE(CAST(json_extract(form_data, '$.season') AS TEXT), '') <> ''
ORDER BY value;

-- name: ProjectEpisodes :many
SELECT DISTINCT CAST(json_extract(form_data, '$.episode') AS TEXT) AS value
FROM uploads
WHERE status = 'completed' AND form_data IS NOT NULL
  AND CAST(json_extract(form_data, '$.project') AS TEXT) = @project
  AND COALESCE(CAST(json_extract(form_data, '$.episode') AS TEXT), '') <> ''
ORDER BY value;

-- name: GetUpload :one
SELECT * FROM uploads WHERE id = @id;

-- name: UpdateUploadOffset :exec
UPDATE uploads SET offset = @offset WHERE id = @id;

-- name: CompleteUpload :exec
UPDATE uploads
SET status = 'completed',
    offset = size,
    duration_ms = CAST((julianday(CURRENT_TIMESTAMP) - julianday(created_at)) * 86400000 AS INTEGER),
    completed_at = CURRENT_TIMESTAMP
WHERE id = @id;

-- name: UpdateDurationMs :exec
UPDATE uploads SET duration_ms = @duration_ms WHERE id = @id;

-- name: FailUpload :exec
UPDATE uploads SET status = 'failed', storage_status = 'failed' WHERE id = @id;

-- name: MarkUploadStorageReady :exec
UPDATE uploads SET storage_status = 'ready' WHERE id = @id;

-- name: FinalizeUploadStorage :one
-- The final filename and readiness are one state transition: a ready row must
-- never point at the pre-sanitised/pre-deduplicated name.
UPDATE uploads
SET filename = @filename, storage_status = 'ready'
WHERE id = @id AND status = 'completed' AND storage_status = 'pending'
RETURNING *;

-- name: ListPendingStorageUploads :many
-- Completed TUS transfers whose asynchronous filesystem/S3 promotion did not
-- finish before the previous process stopped.
SELECT * FROM uploads
WHERE is_partial = 0 AND status = 'completed' AND storage_status = 'pending'
ORDER BY completed_at, created_at, id;

-- name: UpdateUploadFilename :exec
UPDATE uploads SET filename = @filename WHERE id = @id;

-- name: ListUploads :many
SELECT * FROM uploads WHERE is_partial = 0 AND status = 'completed' AND user_id = @user_id ORDER BY created_at DESC;

-- name: ListCompletedFormUploads :many
SELECT id, filename, size, target_name, user_id, completed_at, created_at
FROM uploads
WHERE is_partial = 0 AND status = 'completed' AND form_data IS NOT NULL
ORDER BY completed_at DESC
LIMIT 100;

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = @id;

-- name: DeletePartialUploads :exec
DELETE FROM uploads WHERE final_upload_id = @final_upload_id;
