-- +goose Up
CREATE TABLE teams (
    id INTEGER PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(255)
);

-- +goose Down
DROP TABLE teams;
