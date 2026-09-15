-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Guard: ensure account_locked has no NULL values for rows that predate
-- the 20201201000000_0.11.0_account_locked migration (e.g. manually-created DBs).
-- Setting NULL -> 0 (false) is safe — a NULL account_locked should never lock anyone out.
UPDATE users SET account_locked = 0 WHERE account_locked IS NULL;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
