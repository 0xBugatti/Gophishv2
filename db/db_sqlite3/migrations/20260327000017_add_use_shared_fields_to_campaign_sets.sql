
-- +goose Up
-- Add missing use_shared_* granular flags to campaign_sets.
-- Migration 20250511000002 only added use_shared_settings; the CampaignSet struct
-- has 6 additional per-field flags that were never persisted.
ALTER TABLE campaign_sets ADD COLUMN use_shared_page     BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_url      BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_urlparam BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_qrsize   BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_httpauth BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE campaign_sets ADD COLUMN use_shared_schedule BOOLEAN NOT NULL DEFAULT 0;

-- Add all use_shared_* columns to draft_campaign_sets (none were present in the
-- original 20250510000002 creation DDL).
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_settings BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_page     BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_url      BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_urlparam BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_qrsize   BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_httpauth BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE draft_campaign_sets ADD COLUMN use_shared_schedule BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN
