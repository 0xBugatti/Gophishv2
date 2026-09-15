-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS url_templates (
    id INTEGER PRIMARY KEY AUTO_INCREMENT,
    user_id INTEGER,
    name VARCHAR(255),
    url TEXT,
    category VARCHAR(255),
    is_preset BOOLEAN DEFAULT 0,
    modified_date DATETIME
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE url_templates;
