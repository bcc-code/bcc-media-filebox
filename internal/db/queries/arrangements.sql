-- name: ListArrangements :many
SELECT * FROM arrangements ORDER BY name;

-- name: GetArrangement :one
SELECT * FROM arrangements WHERE id = ?;

-- name: GetArrangementByCode :one
SELECT * FROM arrangements WHERE code = ?;

-- name: CreateArrangement :one
INSERT INTO arrangements (name, code) VALUES (?, ?) RETURNING *;

-- name: UpdateArrangement :one
UPDATE arrangements SET name = ?, code = ? WHERE id = ? RETURNING *;

-- name: DeleteArrangement :exec
DELETE FROM arrangements WHERE id = ?;

-- name: ListSubEvents :many
SELECT * FROM sub_events ORDER BY arrangement_id, name;

-- name: ListSubEventsByArrangement :many
SELECT * FROM sub_events WHERE arrangement_id = ? ORDER BY name;

-- name: ListSubEventsByArrangementCode :many
SELECT se.* FROM sub_events se
JOIN arrangements a ON a.id = se.arrangement_id
WHERE a.code = ?
ORDER BY se.name;

-- name: GetSubEvent :one
SELECT * FROM sub_events WHERE id = ?;

-- name: CreateSubEvent :one
INSERT INTO sub_events (arrangement_id, name, code) VALUES (?, ?, ?) RETURNING *;

-- name: UpdateSubEvent :one
UPDATE sub_events SET name = ?, code = ? WHERE id = ? RETURNING *;

-- name: DeleteSubEvent :exec
DELETE FROM sub_events WHERE id = ?;

-- name: DeleteSubEventsByArrangement :exec
DELETE FROM sub_events WHERE arrangement_id = ?;
