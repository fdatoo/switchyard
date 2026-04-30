-- +goose Up
CREATE TABLE devices (
  id                  TEXT PRIMARY KEY,
  driver_instance_id  TEXT NOT NULL REFERENCES driver_instances(id) ON DELETE RESTRICT,
  friendly_name       TEXT NOT NULL,
  manufacturer        TEXT,
  model               TEXT,
  sw_version          TEXT,
  metadata            BLOB,
  disabled            INTEGER NOT NULL DEFAULT 0,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);
CREATE INDEX devices_driver ON devices(driver_instance_id);

-- +goose Down
DROP TABLE devices;
