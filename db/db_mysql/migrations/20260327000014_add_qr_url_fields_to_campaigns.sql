
-- +goose Up
-- Add urlparam, qrsize, and basicauth fields to campaigns table to match Campaign struct
ALTER TABLE campaigns ADD COLUMN urlparam TEXT NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN qrsize TEXT NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN basicauth BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE campaigns DROP COLUMN urlparam;
ALTER TABLE campaigns DROP COLUMN qrsize;
ALTER TABLE campaigns DROP COLUMN basicauth;
