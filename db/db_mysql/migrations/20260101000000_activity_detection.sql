-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results ADD COLUMN activity_detected BOOLEAN DEFAULT 0;

-- +goose Down
ALTER TABLE results DROP COLUMN activity_detected;
