-- name: CreateShare :one
INSERT INTO shares (id, package_id, upload_id)
VALUES (@id, @package_id, @upload_id)
RETURNING *;

-- name: GetShareByID :one
SELECT * FROM shares WHERE id = @id;

-- name: IncrementShareAccessCount :one
UPDATE shares
SET access_count = access_count + 1
WHERE id = @id
RETURNING *;

-- name: IncrementShareAccessCountIfUnderLimit :one
UPDATE shares
SET access_count = access_count + 1
WHERE id = @id AND access_count < @max_access_count
RETURNING *;
