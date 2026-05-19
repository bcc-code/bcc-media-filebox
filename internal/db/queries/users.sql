-- name: UpsertUser :one
-- Inserts a new user on first login or refreshes profile fields on return
-- visits. The role column is deliberately NOT updated on conflict so that
-- admin-assigned roles survive subsequent logins.
INSERT INTO users (provider, subject, email, name)
VALUES (?, ?, ?, ?)
ON CONFLICT(provider, subject) DO UPDATE SET
    email = excluded.email,
    name = excluded.name,
    last_login_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: SetUserRole :exec
UPDATE users SET role = ? WHERE id = ?;

-- name: ListUsers :many
SELECT * FROM users ORDER BY last_login_at DESC;

-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByProviderSubject :one
SELECT * FROM users WHERE provider = ? AND subject = ?;
