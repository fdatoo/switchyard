-- +goose Up
CREATE TABLE event_subscriptions (
  name         TEXT PRIMARY KEY,
  cursor       INTEGER NOT NULL,
  filter       BLOB,    -- reserved: serialised Filter proto (future server-side filtering)
  created_at   INTEGER NOT NULL,
  last_active  INTEGER NOT NULL
);

-- +goose Down
DROP TABLE event_subscriptions;
