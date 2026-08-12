-- name: CreateShare :one
INSERT INTO shares (id, package_id, upload_id)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetShareByID :one
SELECT * FROM shares WHERE id = ?;
