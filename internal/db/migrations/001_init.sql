CREATE TABLE IF NOT EXISTS uploads (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    size INTEGER NOT NULL,
    offset INTEGER NOT NULL DEFAULT 0,
    content_type TEXT,
    status TEXT NOT NULL DEFAULT 'uploading',
    is_partial INTEGER NOT NULL DEFAULT 0,
    final_upload_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);
