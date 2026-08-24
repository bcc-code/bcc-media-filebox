-- name: ListProjects :many
SELECT * FROM projects ORDER BY name;

-- name: GetProject :one
SELECT * FROM projects WHERE id = @id;

-- name: GetProjectByCode :one
SELECT * FROM projects WHERE code = @code;

-- name: CreateProject :one
INSERT INTO projects (name, code) VALUES (@name, @code) RETURNING *;

-- name: UpdateProject :one
UPDATE projects SET name = @name, code = @code WHERE id = @id RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = @id;
