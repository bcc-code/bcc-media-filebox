-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, expires_at)
VALUES (@id, @user_id, @expires_at);

-- name: GetSessionWithUser :one
SELECT
    s.id AS session_id,
    s.expires_at,
    u.id AS user_id,
    u.provider,
    u.subject,
    u.email,
    u.name,
    u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id = @id AND s.expires_at > CURRENT_TIMESTAMP;

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = @expires_at WHERE id = @id;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = @id;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP;
