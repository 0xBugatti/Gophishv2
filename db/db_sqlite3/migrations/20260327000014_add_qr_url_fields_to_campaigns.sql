
-- +goose Up
-- Add urlparam, qrsize, and basicauth fields to campaigns table to match Campaign struct
ALTER TABLE campaigns ADD COLUMN urlparam TEXT NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN qrsize TEXT NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN basicauth BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN
