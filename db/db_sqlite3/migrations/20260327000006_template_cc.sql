-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- CC recipients on email templates (3.1).
-- Stored as comma-separated RFC 5322 addresses, applied to every email sent with this template.
ALTER TABLE templates ADD COLUMN cc TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
