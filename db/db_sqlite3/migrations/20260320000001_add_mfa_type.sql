-- +goose Up
ALTER TABLE pages ADD COLUMN mfa_type VARCHAR(20) DEFAULT 'sms';
ALTER TABLE pages ADD COLUMN mfa_email_profile_id INTEGER DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN natively
