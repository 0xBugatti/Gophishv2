
-- +goose Up
ALTER TABLE imap ADD COLUMN tracking_type INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE imap DROP COLUMN tracking_type;
