
-- +goose Up
-- 3.8: A/B testing template variants (JSON array of {template_id, weight})
ALTER TABLE campaigns ADD COLUMN template_variants TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN
