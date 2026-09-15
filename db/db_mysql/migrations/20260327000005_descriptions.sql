-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

ALTER TABLE campaigns  ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE templates  ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE pages      ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
