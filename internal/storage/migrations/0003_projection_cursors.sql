-- +goose Up
CREATE TABLE projection_cursors (
  name        TEXT PRIMARY KEY,
  position    INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);

-- +goose Down
DROP TABLE projection_cursors;
