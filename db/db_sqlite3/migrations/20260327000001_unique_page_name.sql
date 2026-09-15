-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Remove any duplicate (user_id, name) rows keeping the most recent one,
-- then enforce uniqueness so concurrent POST /pages requests can't create duplicates.
DELETE FROM pages
WHERE id NOT IN (
    SELECT MAX(id)
    FROM pages
    GROUP BY user_id, name
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pages_user_id_name ON pages (user_id, name);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX IF EXISTS idx_pages_user_id_name;
