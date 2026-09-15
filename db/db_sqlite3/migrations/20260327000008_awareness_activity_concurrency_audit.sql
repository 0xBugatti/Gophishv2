
-- +goose Up
-- 7.5: Awareness page shown to targets after credential capture
ALTER TABLE pages ADD COLUMN awareness_page_html TEXT NOT NULL DEFAULT '';

-- 7.11: User last-activity timestamp (debounced per minute in middleware)
ALTER TABLE users ADD COLUMN last_activity DATETIME;

-- 2.4: Per-campaign parallel sending concurrency
ALTER TABLE campaigns ADD COLUMN send_concurrency INTEGER NOT NULL DEFAULT 1;

-- 6.12: Full audit log table
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    action VARCHAR(32) NOT NULL,
    object_type VARCHAR(128) NOT NULL DEFAULT '',
    object_id INTEGER NOT NULL DEFAULT 0,
    timestamp DATETIME NOT NULL,
    details TEXT NOT NULL DEFAULT ''
);

-- +goose Down
-- SQLite does not support DROP COLUMN; recreating tables would be needed for a
-- full down migration. For safety, we only drop the new table.
DROP TABLE IF EXISTS audit_log;
