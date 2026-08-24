-- name: ListGroups :many
SELECT
    g.id,
    g.name,
    g.kind,
    g.description,
    g.created_at,
    CAST(COUNT(gm.email) AS INTEGER) AS member_count
FROM groups g
LEFT JOIN group_members gm ON gm.group_id = g.id
GROUP BY g.id
ORDER BY g.kind, g.name;

-- name: GetGroup :one
SELECT * FROM groups WHERE id = @id;

-- name: GetGroupByName :one
SELECT * FROM groups WHERE name = @name;

-- name: ListGroupMembers :many
SELECT email FROM group_members WHERE group_id = @group_id ORDER BY email;

-- name: CreateGroup :one
INSERT INTO groups (name, kind, description) VALUES (@name, 'custom', @description) RETURNING *;

-- name: UpdateGroup :one
UPDATE groups SET name = @name, description = @description WHERE id = @id AND kind = 'custom' RETURNING *;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = @id AND kind = 'custom';

-- name: ClearGroupMembers :exec
DELETE FROM group_members WHERE group_id = @group_id;

-- name: AddGroupMember :exec
INSERT INTO group_members (group_id, email) VALUES (@group_id, @email);

-- name: ListGroupNamesForEmail :many
-- All custom + builtin group names whose membership matches the given user.
-- Built-in groups are matched here by their hard-coded wildcard rules so that
-- the Users drawer can show every group a user belongs to.
SELECT g.name FROM groups g
WHERE
    (g.kind = 'custom' AND EXISTS (
        SELECT 1 FROM group_members gm
        WHERE gm.group_id = g.id AND lower(gm.email) = lower(@email)
    ))
    OR (g.name = 'All BCC members'         AND CAST(@provider AS TEXT) = 'bcc')
    OR (g.name = 'All bcc.media employees' AND CAST(@provider AS TEXT) = 'azure')
    OR (g.name = 'All guests'               AND CAST(@provider AS TEXT) = 'guest')
ORDER BY g.kind, g.name;
