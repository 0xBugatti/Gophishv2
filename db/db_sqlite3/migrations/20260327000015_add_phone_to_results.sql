
-- +goose Up
-- Add phone and custom fields to results table to match BaseRecipient struct
ALTER TABLE results ADD COLUMN phone VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE results ADD COLUMN custom TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN
