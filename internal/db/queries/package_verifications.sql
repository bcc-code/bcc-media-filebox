-- name: CreatePackageVerification :one
INSERT INTO package_verifications (id, package_id, expires_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetValidPackageVerification :one
SELECT * FROM package_verifications
WHERE id = ? AND package_id = ? AND expires_at > CURRENT_TIMESTAMP;
