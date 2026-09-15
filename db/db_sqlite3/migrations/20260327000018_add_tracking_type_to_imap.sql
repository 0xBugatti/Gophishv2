
-- +goose Up
-- Add tracking_type to imap table (0=report, 1=reply).
-- The IMAP struct has this field but it was never added to the schema.
ALTER TABLE imap ADD COLUMN tracking_type INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN
