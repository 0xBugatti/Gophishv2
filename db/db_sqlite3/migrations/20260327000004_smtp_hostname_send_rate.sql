-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- smtp_hostname: custom EHLO/HELO value override for sending profiles (3.7)
ALTER TABLE smtp ADD COLUMN smtp_hostname VARCHAR(255) NOT NULL DEFAULT '';

-- send_rate: emails per minute limit per sending profile (2.2, 0 = unlimited)
ALTER TABLE smtp ADD COLUMN send_rate INTEGER NOT NULL DEFAULT 0;

-- send_interval_ms: fixed delay between email dispatches per campaign (2.2)
ALTER TABLE campaigns ADD COLUMN send_interval_ms INTEGER NOT NULL DEFAULT 0;

-- randomize_send_order: Fisher-Yates shuffle targets before dispatch (2.5)
ALTER TABLE campaigns ADD COLUMN randomize_send_order BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
