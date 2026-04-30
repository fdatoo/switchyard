-- +goose Up
CREATE TABLE snapshots (
  position    INTEGER PRIMARY KEY,
  ts          INTEGER NOT NULL,
  owner       TEXT    NOT NULL,
  encoding    TEXT    NOT NULL,
  state       BLOB    NOT NULL,
  meta        BLOB
);
CREATE INDEX snapshots_owner ON snapshots(owner, position DESC);

-- +goose Down
DROP TABLE snapshots;
