
-- +goose Up
-- Add phone and custom fields to targets table to match BaseRecipient struct
ALTER TABLE targets ADD COLUMN phone VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE targets ADD COLUMN custom TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE targets DROP COLUMN phone;
ALTER TABLE targets DROP COLUMN custom;
