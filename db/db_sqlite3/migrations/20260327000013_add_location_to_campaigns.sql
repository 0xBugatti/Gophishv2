
-- +goose Up
-- Add location field to campaigns table to match Campaign struct
ALTER TABLE campaigns ADD COLUMN location VARCHAR(255) NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN
