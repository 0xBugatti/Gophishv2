-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Server-side session table for cryptographic logout invalidation.
-- Each login stores a unique session token; logout deletes the row so
-- reuse of a stolen cookie is rejected in middleware (fix for issue #1.1).
CREATE TABLE IF NOT EXISTS sessions (
    id          INT          NOT NULL AUTO_INCREMENT,
    user_id     INT          NOT NULL,
    token       VARCHAR(128) NOT NULL UNIQUE,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_token   ON sessions (token);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS sessions;
