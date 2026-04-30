-- +goose Up
CREATE TABLE events (
  position        INTEGER PRIMARY KEY AUTOINCREMENT,
  ts              INTEGER NOT NULL,
  kind            TEXT    NOT NULL,
  entity          TEXT,
  source          TEXT    NOT NULL,
  correlation_id  BLOB,
  cause_position  INTEGER,
  payload         BLOB    NOT NULL
);
CREATE INDEX events_ts          ON events(ts);
CREATE INDEX events_entity_ts   ON events(entity, ts)        WHERE entity IS NOT NULL;
CREATE INDEX events_kind_ts     ON events(kind, ts);
CREATE INDEX events_correlation ON events(correlation_id)    WHERE correlation_id IS NOT NULL;
CREATE INDEX events_cause       ON events(cause_position)    WHERE cause_position IS NOT NULL;

-- +goose Down
DROP TABLE events;
