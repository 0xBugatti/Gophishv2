-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

ALTER TABLE templates ADD COLUMN tracking_method VARCHAR(16) NOT NULL DEFAULT 'pixel';
ALTER TABLE smtp ADD COLUMN enable_smtputf8 TINYINT(1) NOT NULL DEFAULT 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

