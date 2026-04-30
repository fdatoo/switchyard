-- +goose Up
CREATE TABLE entities (
  id                  TEXT PRIMARY KEY,
  device_id           TEXT REFERENCES devices(id) ON DELETE SET NULL,
  driver_instance_id  TEXT NOT NULL REFERENCES driver_instances(id) ON DELETE RESTRICT,
  entity_type         TEXT NOT NULL,
  friendly_name       TEXT NOT NULL,
  capabilities        BLOB NOT NULL,
  disabled            INTEGER NOT NULL DEFAULT 0,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);
CREATE INDEX entities_type   ON entities(entity_type);
CREATE INDEX entities_device ON entities(device_id) WHERE device_id IS NOT NULL;
CREATE INDEX entities_driver ON entities(driver_instance_id);

-- +goose Down
DROP TABLE entities;
