-- +goose Up
CREATE TABLE teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(255)
);

-- +goose Down
DROP TABLE teams;
