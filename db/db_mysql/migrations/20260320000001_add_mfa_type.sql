-- +goose Up
ALTER TABLE pages ADD COLUMN mfa_type VARCHAR(20) DEFAULT 'sms';
ALTER TABLE pages ADD COLUMN mfa_email_profile_id BIGINT DEFAULT 0;

-- +goose Down
ALTER TABLE pages DROP COLUMN mfa_type;
ALTER TABLE pages DROP COLUMN mfa_email_profile_id;
