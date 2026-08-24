-- +goose Up
-- An upload is "completed" as soon as tus has received every byte, but moving
-- it to its final filesystem target (or uploading it to S3) happens
-- asynchronously. Keep that second state explicit so package preparation never
-- starts reading an object that is not there yet.
ALTER TABLE uploads ADD COLUMN storage_status TEXT NOT NULL DEFAULT 'ready'
    CHECK (storage_status IN ('pending', 'ready', 'failed'));

UPDATE uploads
SET storage_status = CASE status
	WHEN 'completed' THEN CASE WHEN is_partial = 0 THEN 'ready' ELSE 'pending' END
    WHEN 'failed' THEN 'failed'
    ELSE 'pending'
END;

-- Existing packages are already downloadable and therefore start ready. New
-- packages switch to processing only when their artifact plan is persisted.
ALTER TABLE packages ADD COLUMN preparation_status TEXT NOT NULL DEFAULT 'ready'
    CHECK (preparation_status IN ('ready', 'processing', 'failed'));
ALTER TABLE packages ADD COLUMN preparation_bytes_total INTEGER NOT NULL DEFAULT 0
    CHECK (preparation_bytes_total >= 0);
ALTER TABLE packages ADD COLUMN preparation_bytes_done INTEGER NOT NULL DEFAULT 0
    CHECK (preparation_bytes_done >= 0);
ALTER TABLE packages ADD COLUMN preparation_error TEXT;

-- A package artifact is one recipient-visible download. A direct file has one
-- member; a zip has one or more. source_size drives preparation progress while
-- size is the finished object's actual downloadable size.
CREATE TABLE package_artifacts (
    id             TEXT PRIMARY KEY,
    package_id     TEXT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL CHECK (kind IN ('file', 'zip')),
    filename       TEXT NOT NULL,
    size           INTEGER NOT NULL DEFAULT 0 CHECK (size >= 0),
    source_size    INTEGER NOT NULL CHECK (source_size >= 0),
    position       INTEGER NOT NULL CHECK (position >= 0),
    status         TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'building', 'ready', 'failed')),
    progress_bytes INTEGER NOT NULL DEFAULT 0 CHECK (progress_bytes >= 0),
    access_count   INTEGER NOT NULL DEFAULT 0 CHECK (access_count >= 0),
    attempts       INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    object_key     TEXT,
    error          TEXT,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_package_artifacts_package_position
    ON package_artifacts(package_id, position);
CREATE INDEX idx_package_artifacts_status
    ON package_artifacts(status, created_at);

CREATE TABLE package_artifact_members (
    artifact_id     TEXT NOT NULL REFERENCES package_artifacts(id) ON DELETE CASCADE,
    share_id        TEXT NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL CHECK (position >= 0),
    archive_filename TEXT NOT NULL,
    PRIMARY KEY (artifact_id, share_id),
    UNIQUE (artifact_id, position),
    UNIQUE (share_id)
);

-- Preserve the old one-download-per-share behaviour, including access budgets.
-- LEFT JOIN also retains an old orphaned share: it remains visible but, as
-- before this migration, attempting to fetch its missing upload will fail.
INSERT INTO package_artifacts (
    id, package_id, kind, filename, size, source_size, position, status,
    progress_bytes, access_count, attempts, object_key, error, created_at, updated_at
)
SELECT
    'legacy-' || s.id,
    s.package_id,
    'file',
    COALESCE(u.filename, s.id),
    COALESCE(u.size, 0),
    COALESCE(u.size, 0),
    ROW_NUMBER() OVER (
        PARTITION BY s.package_id
        ORDER BY s.created_at, s.id
    ) - 1,
    'ready',
    COALESCE(u.size, 0),
    s.access_count,
    0,
    NULL,
    NULL,
    s.created_at,
    s.created_at
FROM shares s
LEFT JOIN uploads u ON u.id = s.upload_id;

INSERT INTO package_artifact_members (artifact_id, share_id, position, archive_filename)
SELECT
    'legacy-' || s.id,
    s.id,
    0,
    COALESCE(u.filename, s.id)
FROM shares s
LEFT JOIN uploads u ON u.id = s.upload_id;

-- +goose Down
DROP TABLE IF EXISTS package_artifact_members;
DROP INDEX IF EXISTS idx_package_artifacts_status;
DROP INDEX IF EXISTS idx_package_artifacts_package_position;
DROP TABLE IF EXISTS package_artifacts;

ALTER TABLE packages DROP COLUMN preparation_error;
ALTER TABLE packages DROP COLUMN preparation_bytes_done;
ALTER TABLE packages DROP COLUMN preparation_bytes_total;
ALTER TABLE packages DROP COLUMN preparation_status;
ALTER TABLE uploads DROP COLUMN storage_status;
