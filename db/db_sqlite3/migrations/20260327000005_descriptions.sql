-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Add optional description fields to campaigns, templates, and landing pages (3.12).
-- Descriptions are purely informational and are never sent to targets.
ALTER TABLE campaigns  ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE templates  ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE pages      ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
