-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Remove any duplicate (user_id, name) rows keeping the most recent one,
-- then enforce uniqueness so concurrent POST /pages requests can't create duplicates.
DELETE p1 FROM pages p1
INNER JOIN pages p2
WHERE p1.id < p2.id AND p1.user_id = p2.user_id AND p1.name = p2.name;

CREATE UNIQUE INDEX idx_pages_user_id_name ON pages (user_id, name);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX idx_pages_user_id_name ON pages;
