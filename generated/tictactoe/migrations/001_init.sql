-- Tic-Tac-Toe database schema
-- Currently using in-memory event store, so no migrations needed

-- If switching to SQLite persistence:
-- CREATE TABLE IF NOT EXISTS events (
--     id TEXT PRIMARY KEY,
--     stream_id TEXT NOT NULL,
--     version INTEGER NOT NULL,
--     type TEXT NOT NULL,
--     data BLOB,
--     timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
--     UNIQUE(stream_id, version)
-- );

-- CREATE INDEX idx_events_stream ON events(stream_id);
