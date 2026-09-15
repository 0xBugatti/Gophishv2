
-- +goose Up
-- 7.5: Awareness page shown to targets after credential capture
ALTER TABLE pages ADD COLUMN awareness_page_html TEXT NOT NULL;

-- 7.11: User last-activity timestamp (debounced per minute in middleware)
ALTER TABLE users ADD COLUMN last_activity DATETIME NULL;

-- 2.4: Per-campaign parallel sending concurrency
ALTER TABLE campaigns ADD COLUMN send_concurrency INT NOT NULL DEFAULT 1;

-- 6.12: Full audit log table
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    action VARCHAR(32) NOT NULL,
    object_type VARCHAR(128) NOT NULL DEFAULT '',
    object_id BIGINT NOT NULL DEFAULT 0,
    timestamp DATETIME NOT NULL,
    details TEXT NOT NULL,
    INDEX idx_audit_timestamp (timestamp),
    INDEX idx_audit_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
ALTER TABLE pages DROP COLUMN awareness_page_html;
ALTER TABLE users DROP COLUMN last_activity;
ALTER TABLE campaigns DROP COLUMN send_concurrency;
DROP TABLE IF EXISTS audit_log;
