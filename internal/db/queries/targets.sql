-- name: ListTargets :many
SELECT * FROM targets ORDER BY position, id;

-- name: GetTarget :one
SELECT * FROM targets WHERE id = ?;

-- name: GetTargetByName :one
SELECT * FROM targets WHERE name = ?;

-- name: CreateTarget :one
INSERT INTO targets (name, path, form_key, webhook_url, position)
VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(position), 0) + 1 FROM targets))
RETURNING *;

-- name: UpdateTarget :one
UPDATE targets SET name = ?, path = ?, form_key = ?, webhook_url = ? WHERE id = ? RETURNING *;

-- name: UpdateTargetPosition :exec
UPDATE targets SET position = ? WHERE id = ?;

-- name: DeleteTarget :exec
DELETE FROM targets WHERE id = ?;

-- name: CountTargets :one
SELECT COUNT(*) FROM targets;
