-- +goose Up
CREATE TABLE driver_instances (
  id             TEXT PRIMARY KEY,
  driver_name    TEXT    NOT NULL,
  display_name   TEXT    NOT NULL,
  transport      TEXT    NOT NULL CHECK(transport IN ('local_subprocess','remote_grpc')),
  endpoint       TEXT    NOT NULL,
  config_hash    TEXT    NOT NULL,
  status         TEXT    NOT NULL CHECK(status IN ('starting','running','failed','stopped')),
  last_error     TEXT,
  started_at     INTEGER,
  last_heartbeat INTEGER,
  created_at     INTEGER NOT NULL
);

-- +goose Down
DROP TABLE driver_instances;
