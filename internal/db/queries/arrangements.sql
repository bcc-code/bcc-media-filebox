-- name: ListArrangements :many
SELECT * FROM arrangements ORDER BY name;

-- name: GetArrangement :one
SELECT * FROM arrangements WHERE id = @id;

-- name: GetArrangementByCode :one
SELECT * FROM arrangements WHERE code = @code;

-- name: CreateArrangement :one
INSERT INTO arrangements (name, code) VALUES (@name, @code) RETURNING *;

-- name: UpdateArrangement :one
UPDATE arrangements SET name = @name, code = @code WHERE id = @id RETURNING *;

-- name: DeleteArrangement :exec
DELETE FROM arrangements WHERE id = @id;

-- name: ListSubEvents :many
SELECT * FROM sub_events ORDER BY arrangement_id, name;

-- name: ListSubEventsByArrangement :many
SELECT * FROM sub_events WHERE arrangement_id = @arrangement_id ORDER BY name;

-- name: ListSubEventsByArrangementCode :many
SELECT se.* FROM sub_events se
JOIN arrangements a ON a.id = se.arrangement_id
WHERE a.code = @code
ORDER BY se.name;

-- name: GetSubEvent :one
SELECT * FROM sub_events WHERE id = @id;

-- name: CreateSubEvent :one
INSERT INTO sub_events (arrangement_id, name, code) VALUES (@arrangement_id, @name, @code) RETURNING *;

-- name: UpdateSubEvent :one
UPDATE sub_events SET name = @name, code = @code WHERE id = @id RETURNING *;

-- name: DeleteSubEvent :exec
DELETE FROM sub_events WHERE id = @id;

-- name: DeleteSubEventsByArrangement :exec
DELETE FROM sub_events WHERE arrangement_id = @arrangement_id;
