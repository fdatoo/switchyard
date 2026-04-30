-- +goose Up
CREATE TABLE skipped_events (
  position     INTEGER NOT NULL,
  projector    TEXT    NOT NULL,
  skipped_at   INTEGER NOT NULL,
  skipped_by   TEXT    NOT NULL,
  reason       TEXT    NOT NULL,
  PRIMARY KEY (position, projector)
);

-- +goose Down
DROP TABLE skipped_events;
