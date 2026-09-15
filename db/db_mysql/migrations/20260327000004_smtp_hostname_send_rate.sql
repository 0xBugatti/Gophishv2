-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

ALTER TABLE smtp ADD COLUMN smtp_hostname VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE smtp ADD COLUMN send_rate INT NOT NULL DEFAULT 0;
ALTER TABLE campaigns ADD COLUMN send_interval_ms INT NOT NULL DEFAULT 0;
ALTER TABLE campaigns ADD COLUMN randomize_send_order TINYINT(1) NOT NULL DEFAULT 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
