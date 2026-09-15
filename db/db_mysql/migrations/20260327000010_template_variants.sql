
-- +goose Up
-- 3.8: A/B testing template variants (JSON array of {template_id, weight})
ALTER TABLE campaigns ADD COLUMN template_variants TEXT NOT NULL;

-- +goose Down
ALTER TABLE campaigns DROP COLUMN template_variants;
