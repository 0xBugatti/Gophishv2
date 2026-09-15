
-- +goose Up
-- Add fields to email_requests table to match EmailRequest struct (BaseRecipient + EncryptionKey)
ALTER TABLE email_requests ADD COLUMN encryption_key VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN phone VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN custom TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE email_requests DROP COLUMN encryption_key;
ALTER TABLE email_requests DROP COLUMN phone;
ALTER TABLE email_requests DROP COLUMN custom;
