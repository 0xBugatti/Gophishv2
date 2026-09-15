
-- +goose Up
-- 4.7: Configurable RId charset and length per campaign
ALTER TABLE campaigns ADD COLUMN rid_charset VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE campaigns ADD COLUMN rid_length INT NOT NULL DEFAULT 0;

-- 7.6: Auto-submit countdown timer on landing pages
ALTER TABLE pages ADD COLUMN auto_submit_delay INT NOT NULL DEFAULT 0;

-- 7.7: Multi-page landing flow (next page chaining)
ALTER TABLE pages ADD COLUMN next_page_id BIGINT NOT NULL DEFAULT 0;

-- 3.11: Credential passthrough to real site
ALTER TABLE pages ADD COLUMN passthrough_url VARCHAR(2048) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE campaigns DROP COLUMN rid_charset;
ALTER TABLE campaigns DROP COLUMN rid_length;
ALTER TABLE pages DROP COLUMN auto_submit_delay;
ALTER TABLE pages DROP COLUMN next_page_id;
ALTER TABLE pages DROP COLUMN passthrough_url;
