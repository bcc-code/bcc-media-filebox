-- name: ListProjects :many
SELECT * FROM projects ORDER BY name;

-- name: GetProject :one
SELECT * FROM projects WHERE id = ?;

-- name: GetProjectByCode :one
SELECT * FROM projects WHERE code = ?;

-- name: CreateProject :one
INSERT INTO projects (name, code) VALUES (?, ?) RETURNING *;

-- name: UpdateProject :one
UPDATE projects SET name = ?, code = ? WHERE id = ? RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = ?;
