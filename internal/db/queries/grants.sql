-- name: ListGrants :many
SELECT
    g.id,
    g.principal_kind,
    g.principal_value,
    g.admin,
    g.all_targets,
    g.created_at,
    CAST(COALESCE(GROUP_CONCAT(gt.target_id), '') AS TEXT) AS target_ids
FROM grants g
LEFT JOIN grant_targets gt ON gt.grant_id = g.id
GROUP BY g.id
ORDER BY g.created_at;

-- name: GetGrant :one
SELECT
    g.id,
    g.principal_kind,
    g.principal_value,
    g.admin,
    g.all_targets,
    g.created_at,
    CAST(COALESCE(GROUP_CONCAT(gt.target_id), '') AS TEXT) AS target_ids
FROM grants g
LEFT JOIN grant_targets gt ON gt.grant_id = g.id
WHERE g.id = ?
GROUP BY g.id;

-- name: GetGrantByPrincipal :one
SELECT * FROM grants
WHERE principal_kind = sqlc.arg(principal_kind)
  AND lower(principal_value) = lower(sqlc.arg(principal_value));

-- name: CreateGrant :one
INSERT INTO grants (principal_kind, principal_value, admin, all_targets)
VALUES (?, ?, ?, ?) RETURNING *;

-- name: UpdateGrant :exec
UPDATE grants SET admin = ?, all_targets = ? WHERE id = ?;

-- name: UpdateGrantPrincipal :exec
UPDATE grants SET principal_kind = ?, principal_value = ? WHERE id = ?;

-- name: DeleteGrant :exec
DELETE FROM grants WHERE id = ?;

-- name: ClearGrantTargets :exec
DELETE FROM grant_targets WHERE grant_id = ?;

-- name: AddGrantTarget :exec
INSERT INTO grant_targets (grant_id, target_id) VALUES (?, ?);

-- name: DeleteGrantsByPrincipal :exec
DELETE FROM grants WHERE principal_kind = ? AND principal_value = ?;

-- name: DeleteGrantsByGroupName :exec
DELETE FROM grants WHERE principal_kind = 'group' AND principal_value = ?;

-- name: CountGrantsByGroupName :one
SELECT COUNT(*) FROM grants WHERE principal_kind = 'group' AND principal_value = ?;

-- name: GrantsForUser :many
-- Returns all grants that apply to (email, provider). Direct user grants match
-- by email; builtin group grants expand by provider/domain wildcard; custom
-- group grants expand by explicit membership.
SELECT
    g.id,
    g.principal_kind,
    g.principal_value,
    g.admin,
    g.all_targets,
    g.created_at,
    CAST(COALESCE(GROUP_CONCAT(gt.target_id), '') AS TEXT) AS target_ids
FROM grants g
LEFT JOIN grant_targets gt ON gt.grant_id = g.id
WHERE
    (g.principal_kind = 'user' AND lower(g.principal_value) = lower(sqlc.arg(email)))
    OR (g.principal_kind = 'group' AND g.principal_value = 'All BCC members'         AND CAST(sqlc.arg(provider) AS TEXT) = 'bcc')
    OR (g.principal_kind = 'group' AND g.principal_value = 'All bcc.media employees' AND CAST(sqlc.arg(provider) AS TEXT) = 'azure')
    OR (g.principal_kind = 'group' AND g.principal_value = 'All guests'               AND CAST(sqlc.arg(provider) AS TEXT) = 'guest')
    OR (g.principal_kind = 'group' AND g.principal_value IN (
        SELECT gr.name FROM groups gr
        JOIN group_members gm ON gm.group_id = gr.id
        WHERE gr.kind = 'custom' AND lower(gm.email) = lower(sqlc.arg(email))
    ))
GROUP BY g.id;
