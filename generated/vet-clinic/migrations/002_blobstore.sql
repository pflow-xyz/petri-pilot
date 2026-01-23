-- Migration: 002_blobstore
-- Blob store for vet clinic attachments (lab reports, X-rays, prescriptions, notes)

CREATE TABLE IF NOT EXISTS blobs (
    id TEXT PRIMARY KEY,
    stream_id TEXT NOT NULL,           -- Links to appointment/workflow instance
    owner_id TEXT NOT NULL,            -- User who uploaded
    filename TEXT NOT NULL,            -- Original filename
    content_type TEXT NOT NULL,        -- MIME type
    size INTEGER NOT NULL,             -- Bytes
    data BLOB NOT NULL,                -- File content
    attachment_type TEXT,              -- lab_report, xray, prescription, note, etc.
    created_at TEXT NOT NULL,
    metadata TEXT                      -- JSON for extra fields
);

CREATE INDEX IF NOT EXISTS idx_blobs_stream ON blobs(stream_id);
CREATE INDEX IF NOT EXISTS idx_blobs_owner ON blobs(owner_id);
CREATE INDEX IF NOT EXISTS idx_blobs_type ON blobs(attachment_type);
CREATE INDEX IF NOT EXISTS idx_blobs_created_at ON blobs(created_at);
