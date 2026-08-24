-- name: ListTargets :many
SELECT * FROM targets ORDER BY position, id;

-- name: GetTarget :one
SELECT * FROM targets WHERE id = @id;

-- name: GetTargetByName :one
SELECT * FROM targets WHERE name = @name;

-- name: CreateTarget :one
INSERT INTO targets (name, path, form_key, webhook_url, position)
VALUES (@name, @path, @form_key, @webhook_url, (SELECT COALESCE(MAX(position), 0) + 1 FROM targets))
RETURNING *;

-- name: UpdateTarget :one
UPDATE targets SET name = @name, path = @path, form_key = @form_key, webhook_url = @webhook_url WHERE id = @id RETURNING *;

-- name: UpdateTargetPosition :exec
UPDATE targets SET position = @position WHERE id = @id;

-- name: DeleteTarget :exec
DELETE FROM targets WHERE id = @id;

-- name: CountTargets :one
SELECT COUNT(*) FROM targets;
