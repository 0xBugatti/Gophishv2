-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE campaigns ADD COLUMN start_time DATETIME;
ALTER TABLE campaigns ADD COLUMN end_time DATETIME;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE campaigns DROP COLUMN start_time;
ALTER TABLE campaigns DROP COLUMN end_time;