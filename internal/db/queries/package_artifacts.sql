-- name: CreatePackageArtifact :one
INSERT INTO package_artifacts (
    id, package_id, kind, filename, size, source_size, position, status,
    progress_bytes, object_key
)
VALUES (@id, @package_id, @kind, @filename, @size, @source_size, @position, @status, @progress_bytes, @object_key)
RETURNING *;

-- name: CreatePackageArtifactMember :one
INSERT INTO package_artifact_members (artifact_id, share_id, position, archive_filename)
VALUES (@artifact_id, @share_id, @position, @archive_filename)
RETURNING *;

-- name: GetPackageArtifact :one
SELECT * FROM package_artifacts WHERE id = @id;

-- name: GetPackageArtifactByShareID :one
-- Legacy share links must resolve through their recipient-visible artifact so
-- preparation state and per-artifact download limits cannot be bypassed.
SELECT a.*
FROM package_artifacts a
JOIN package_artifact_members m ON m.artifact_id = a.id
WHERE m.share_id = @share_id;

-- name: ListPackageArtifacts :many
SELECT * FROM package_artifacts
WHERE package_id = @package_id
ORDER BY position, id;

-- name: ListPackageArtifactsByPackageIDs :many
SELECT * FROM package_artifacts
WHERE package_id IN (sqlc.slice('package_ids'))
ORDER BY package_id, position, id;

-- name: ListPendingPackageArtifacts :many
SELECT * FROM package_artifacts
WHERE package_id = @package_id AND status = 'pending'
ORDER BY position, id;

-- name: GetPackageArtifactMemberWithUpload :one
SELECT
    m.artifact_id,
    m.share_id,
    m.position,
    m.archive_filename,
    s.package_id,
    s.upload_id,
    s.access_count AS share_access_count,
    u.filename AS upload_filename,
    u.size AS upload_size,
    u.content_type AS upload_content_type,
    u.status AS upload_status,
    u.storage_status AS upload_storage_status,
    u.target_name AS upload_target_name,
    u.sha256 AS upload_sha256,
    u.created_at AS upload_created_at,
    u.completed_at AS upload_completed_at
FROM package_artifact_members m
JOIN shares s ON s.id = m.share_id
JOIN uploads u ON u.id = s.upload_id
WHERE m.artifact_id = @artifact_id AND m.share_id = @share_id;

-- name: ListPackageArtifactMembersWithUploads :many
SELECT
    m.artifact_id,
    m.share_id,
    m.position,
    m.archive_filename,
    s.package_id,
    s.upload_id,
    s.access_count AS share_access_count,
    u.filename AS upload_filename,
    u.size AS upload_size,
    u.content_type AS upload_content_type,
    u.status AS upload_status,
    u.storage_status AS upload_storage_status,
    u.target_name AS upload_target_name,
    u.sha256 AS upload_sha256,
    u.created_at AS upload_created_at,
    u.completed_at AS upload_completed_at
FROM package_artifact_members m
JOIN shares s ON s.id = m.share_id
JOIN uploads u ON u.id = s.upload_id
WHERE m.artifact_id = @artifact_id
ORDER BY m.position, m.share_id;

-- name: SetPackagePreparationProcessing :one
UPDATE packages
SET preparation_status = 'processing',
    preparation_bytes_total = @preparation_bytes_total,
    preparation_bytes_done = 0,
    preparation_error = NULL
WHERE id = @id
RETURNING *;

-- name: UpdatePackagePreparationProgress :one
UPDATE packages
SET preparation_bytes_done = @preparation_bytes_done
WHERE id = @id AND preparation_status = 'processing'
RETURNING *;

-- name: FinalizePackagePreparation :one
UPDATE packages
SET preparation_status = 'ready',
    preparation_bytes_done = preparation_bytes_total,
    preparation_error = NULL
WHERE id = @id AND preparation_status = 'processing'
RETURNING *;

-- name: FailPackagePreparation :one
UPDATE packages
SET preparation_status = 'failed', preparation_error = @preparation_error
WHERE id = @id AND preparation_status = 'processing'
RETURNING *;

-- name: ListProcessingPackages :many
SELECT * FROM packages
WHERE preparation_status = 'processing'
ORDER BY created_at, id;

-- name: ListReadyPackagesWithUnsentRecipients :many
SELECT * FROM packages p
WHERE p.preparation_status = 'ready'
  AND p.status = 'active'
  AND p.expires_at > CURRENT_TIMESTAMP
  AND EXISTS (
      SELECT 1
      FROM package_recipients r
      WHERE r.package_id = p.id AND r.sent_at IS NULL
  )
ORDER BY p.created_at, p.id;

-- name: ResetBuildingPackageArtifacts :execrows
UPDATE package_artifacts
SET status = 'pending', progress_bytes = 0, error = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE status = 'building';

-- name: RequeueBuildingPackageArtifact :execrows
UPDATE package_artifacts
SET status = 'pending', progress_bytes = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'building';

-- name: MarkPackageArtifactBuilding :one
UPDATE package_artifacts
SET status = 'building', progress_bytes = 0, attempts = attempts + 1,
    error = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'pending'
RETURNING *;

-- name: UpdatePackageArtifactProgress :one
UPDATE package_artifacts
SET progress_bytes = @progress_bytes, updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'building'
RETURNING *;

-- name: MarkPackageArtifactReady :one
UPDATE package_artifacts
SET status = 'ready', size = @size, object_key = @object_key,
    progress_bytes = source_size, error = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'building'
RETURNING *;

-- name: RetryPackageArtifact :one
UPDATE package_artifacts
SET status = 'pending', progress_bytes = 0, error = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'failed'
RETURNING *;

-- name: FailPackageArtifact :one
UPDATE package_artifacts
SET status = 'failed', error = @error, updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status IN ('pending', 'building')
RETURNING *;

-- name: RequeuePackageArtifactAfterFailure :one
-- Keep retryable failures out of the externally visible failed state. This is
-- one transition so even another worker can never mistake a retry window for a
-- terminal package failure.
UPDATE package_artifacts
SET status = CASE
        WHEN attempts < @max_attempts THEN 'pending'
        ELSE 'failed'
    END,
    progress_bytes = 0,
    error = CASE
        WHEN attempts < @max_attempts THEN NULL
        ELSE @error
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'building'
RETURNING *;

-- name: IncrementPackageArtifactAccessCount :one
UPDATE package_artifacts
SET access_count = access_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'ready'
RETURNING *;

-- name: IncrementPackageArtifactAccessCountIfUnderLimit :one
UPDATE package_artifacts
SET access_count = access_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND status = 'ready' AND access_count < @max_access_count
RETURNING *;

-- name: IncrementArtifactMemberShareAccessCounts :execrows
UPDATE shares
SET access_count = access_count + 1
WHERE id IN (
    SELECT share_id FROM package_artifact_members WHERE artifact_id = @artifact_id
);

-- name: GetPackageArtifactAccessCounts :one
SELECT
    COUNT(*) AS artifact_count,
    CAST(COALESCE(MAX(access_count), 0) AS INTEGER) AS max_access_count,
    CAST(COALESCE(MIN(access_count), 0) AS INTEGER) AS min_access_count
FROM package_artifacts
WHERE package_id = @package_id;

-- name: GetPackageArtifactAccessCountsByPackageIDs :many
SELECT
    package_id,
    COUNT(*) AS artifact_count,
    CAST(COALESCE(MAX(access_count), 0) AS INTEGER) AS max_access_count,
    CAST(COALESCE(MIN(access_count), 0) AS INTEGER) AS min_access_count
FROM package_artifacts
WHERE package_id IN (sqlc.slice('package_ids'))
GROUP BY package_id
ORDER BY package_id;
