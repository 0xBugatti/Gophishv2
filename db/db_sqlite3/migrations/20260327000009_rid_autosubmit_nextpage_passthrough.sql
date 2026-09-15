
-- +goose Up
-- 4.7: Configurable RId charset and length per campaign
ALTER TABLE campaigns ADD COLUMN rid_charset VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN rid_length INTEGER NOT NULL DEFAULT 0;

-- 7.6: Auto-submit countdown timer on landing pages
ALTER TABLE pages ADD COLUMN auto_submit_delay INTEGER NOT NULL DEFAULT 0;

-- 7.7: Multi-page landing flow (next page chaining)
ALTER TABLE pages ADD COLUMN next_page_id INTEGER NOT NULL DEFAULT 0;

-- 3.11: Credential passthrough to real site
ALTER TABLE pages ADD COLUMN passthrough_url VARCHAR(2048) NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN; only drop the new table artifacts.
