-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

ALTER TABLE templates ADD COLUMN cc TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
