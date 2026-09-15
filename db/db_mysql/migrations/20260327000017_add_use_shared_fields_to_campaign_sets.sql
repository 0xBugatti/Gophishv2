
-- +goose Up
-- Add missing use_shared_* granular flags to campaign_sets.
ALTER TABLE campaign_sets ADD COLUMN use_shared_page     BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_url      BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_urlparam BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_qrsize   BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_httpauth BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_schedule BOOLEAN NOT NULL DEFAULT 0;

-- Add all use_shared_* columns to draft_campaign_sets.
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_settings BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_page     BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_url      BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_urlparam BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_qrsize   BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_httpauth BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_schedule BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE campaign_sets DROP COLUMN use_shared_page;
ALTER TABLE campaign_sets DROP COLUMN use_shared_url;
ALTER TABLE campaign_sets DROP COLUMN use_shared_urlparam;
ALTER TABLE campaign_sets DROP COLUMN use_shared_qrsize;
ALTER TABLE campaign_sets DROP COLUMN use_shared_httpauth;
ALTER TABLE campaign_sets DROP COLUMN use_shared_schedule;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_settings;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_page;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_url;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_urlparam;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_qrsize;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_httpauth;
ALTER TABLE draft_campaign_sets DROP COLUMN use_shared_schedule;
