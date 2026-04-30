package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (r *Registry) GetDriverInstance(ctx context.Context, id string) (DriverInstance, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, driver_name, display_name, transport, endpoint, config_hash, status,
		COALESCE(last_error, ''), COALESCE(started_at, 0), COALESCE(last_heartbeat, 0), created_at FROM driver_instances WHERE id = ?`, id)
	return scanDriverInstance(row)
}

func (r *Registry) ListDriverInstances(ctx context.Context) ([]DriverInstance, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, driver_name, display_name, transport, endpoint, config_hash, status,
		COALESCE(last_error, ''), COALESCE(started_at, 0), COALESCE(last_heartbeat, 0), created_at FROM driver_instances ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	out := make([]DriverInstance, 0)
	for rows.Next() {
		di, err := scanDriverInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, di)
	}
	return out, rows.Err()
}

func (r *Registry) GetDevice(ctx context.Context, id string) (Device, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, driver_instance_id, friendly_name,
		COALESCE(manufacturer, ''), COALESCE(model, ''), COALESCE(sw_version, ''),
		metadata, disabled, created_at, updated_at FROM devices WHERE id = ?`, id)
	return scanDevice(row)
}

func (r *Registry) ListDevices(ctx context.Context, f DeviceFilter) ([]Device, error) {
	query := `SELECT id, driver_instance_id, friendly_name,
		COALESCE(manufacturer, ''), COALESCE(model, ''), COALESCE(sw_version, ''),
		metadata, disabled, created_at, updated_at FROM devices WHERE 1=1`
	args := []any{}
	if f.DriverInstanceID != "" {
		query += ` AND driver_instance_id = ?`
		args = append(args, f.DriverInstanceID)
	}
	if !f.IncludeDisabled {
		query += ` AND disabled = 0`
	}
	query += ` ORDER BY id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	out := make([]Device, 0)
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Registry) GetEntity(ctx context.Context, id string) (Entity, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, COALESCE(device_id, ''), driver_instance_id,
		entity_type, friendly_name, capabilities, disabled, created_at, updated_at FROM entities WHERE id = ?`, id)
	return scanEntity(row)
}

func (r *Registry) ListEntities(ctx context.Context, f EntityFilter) ([]Entity, error) {
	query := `SELECT id, COALESCE(device_id, ''), driver_instance_id, entity_type, friendly_name,
		capabilities, disabled, created_at, updated_at FROM entities WHERE 1=1`
	args := []any{}
	if f.DriverInstanceID != "" {
		query += ` AND driver_instance_id = ?`
		args = append(args, f.DriverInstanceID)
	}
	if f.DeviceID != "" {
		query += ` AND device_id = ?`
		args = append(args, f.DeviceID)
	}
	if f.EntityType != "" {
		query += ` AND entity_type = ?`
		args = append(args, f.EntityType)
	}
	if !f.IncludeDisabled {
		query += ` AND disabled = 0`
	}
	query += ` ORDER BY id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	out := make([]Entity, 0)
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanDriverInstance(r scanner) (DriverInstance, error) {
	var di DriverInstance
	var startedAt, lastHB, createdAt int64
	err := r.Scan(&di.ID, &di.DriverName, &di.DisplayName, &di.Transport, &di.Endpoint,
		&di.ConfigHash, &di.Status, &di.LastError, &startedAt, &lastHB, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return di, fmt.Errorf("driver instance not found")
		}
		return di, err
	}
	if startedAt > 0 {
		di.StartedAt = time.Unix(0, startedAt)
	}
	if lastHB > 0 {
		di.LastHeartbeat = time.Unix(0, lastHB)
	}
	di.CreatedAt = time.Unix(0, createdAt)
	return di, nil
}

func scanDevice(r scanner) (Device, error) {
	var d Device
	var disabled int
	var createdAt, updatedAt int64
	err := r.Scan(&d.ID, &d.DriverInstanceID, &d.FriendlyName, &d.Manufacturer, &d.Model,
		&d.SwVersion, &d.Metadata, &disabled, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return d, fmt.Errorf("device not found")
		}
		return d, err
	}
	d.Disabled = disabled != 0
	d.CreatedAt = time.Unix(0, createdAt)
	d.UpdatedAt = time.Unix(0, updatedAt)
	return d, nil
}

func scanEntity(r scanner) (Entity, error) {
	var e Entity
	var disabled int
	var createdAt, updatedAt int64
	err := r.Scan(&e.ID, &e.DeviceID, &e.DriverInstanceID, &e.EntityType, &e.FriendlyName,
		&e.Capabilities, &disabled, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return e, fmt.Errorf("entity not found")
		}
		return e, err
	}
	e.Disabled = disabled != 0
	e.CreatedAt = time.Unix(0, createdAt)
	e.UpdatedAt = time.Unix(0, updatedAt)
	return e, nil
}
