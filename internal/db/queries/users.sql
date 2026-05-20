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

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByProviderSubject :one
SELECT * FROM users WHERE provider = ? AND subject = ?;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower(?) ORDER BY last_login_at DESC LIMIT 1;

-- name: GetNonGuestUserByEmail :one
-- Used by guest sign-up to reject emails already claimed by an OAuth identity.
SELECT * FROM users
WHERE provider != 'guest' AND lower(email) = lower(?)
ORDER BY last_login_at DESC
LIMIT 1;

-- name: UserUploadStats :one
SELECT
    COALESCE(SUM(CASE WHEN status = 'completed' AND is_partial = 0 THEN 1 ELSE 0 END), 0) AS uploads,
    COALESCE(SUM(CASE WHEN status = 'completed' AND is_partial = 0 THEN size ELSE 0 END), 0) AS total_bytes,
    COALESCE(SUM(CASE WHEN status = 'completed' AND is_partial = 0 AND created_at >= date('now', 'start of month') THEN 1 ELSE 0 END), 0) AS uploads_this_month,
    COALESCE(SUM(CASE WHEN status = 'completed' AND is_partial = 0 AND created_at >= date('now', 'start of month') THEN size ELSE 0 END), 0) AS bytes_this_month,
    COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failures
FROM uploads
WHERE user_id = ?;

-- name: UserRecentUploads :many
SELECT id, filename, size, target_name, completed_at, created_at
FROM uploads
WHERE user_id = ? AND status = 'completed' AND is_partial = 0
ORDER BY completed_at DESC
LIMIT 10;

-- name: AggregateUploadStats :one
SELECT
    COALESCE(SUM(CASE WHEN status = 'completed' AND is_partial = 0 THEN 1 ELSE 0 END), 0) AS uploads,
    COALESCE(SUM(CASE WHEN status = 'completed' AND is_partial = 0 THEN size ELSE 0 END), 0) AS total_bytes,
    COALESCE(SUM(CASE WHEN status = 'completed' AND is_partial = 0 AND created_at >= date('now', 'start of month') THEN size ELSE 0 END), 0) AS bytes_this_month
FROM uploads;
