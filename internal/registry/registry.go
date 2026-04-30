package registry

import (
	"context"
	"database/sql"
	"fmt"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
	regMigrations "github.com/fdatoo/gohome/internal/registry/migrations"
	"github.com/fdatoo/gohome/internal/storage"
)

// Registry is the read API and projector for driver instances, devices, and entities.
type Registry struct {
	db *sql.DB
	eventstore.NoSnapshot
}

// New returns a Registry attached to an already-open DB and runs registry migrations.
func New(ctx context.Context, db *sql.DB) (*Registry, error) {
	if err := storage.Migrate(ctx, db, regMigrations.FS, "registry"); err != nil {
		return nil, fmt.Errorf("registry migrations: %w", err)
	}
	return &Registry{db: db}, nil
}

func (r *Registry) Name() string { return "registry" }

// Apply implements eventstore.Projector. Runs inside the Append tx.
func (r *Registry) Apply(ctx context.Context, tx storage.Tx, e eventstore.Event) error {
	switch payload := e.Payload.GetKind().(type) {
	case *eventv1.Payload_EntityRegistered:
		return r.applyEntityRegistered(ctx, tx, e, payload.EntityRegistered)
	case *eventv1.Payload_EntityUnregistered:
		return r.applyEntityUnregistered(ctx, tx, e)
	case *eventv1.Payload_DriverEvent:
		return r.applyDriverEvent(ctx, tx, e, payload.DriverEvent)
	default:
		return nil
	}
}

func (r *Registry) applyEntityRegistered(ctx context.Context, tx storage.Tx, e eventstore.Event, p *eventv1.EntityRegistered) error {
	caps, err := proto.Marshal(p.GetCapabilities())
	if err != nil {
		return fmt.Errorf("marshal capabilities: %w", err)
	}
	now := e.Timestamp.UnixNano()

	// Ensure FK target exists before inserting entity.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO driver_instances
			(id, driver_name, display_name, transport, endpoint, config_hash, status, created_at)
		VALUES (?, '', '', 'local_subprocess', '', '', 'starting', ?)
		ON CONFLICT(id) DO NOTHING`,
		p.DriverInstanceId, now,
	); err != nil {
		return fmt.Errorf("ensure driver_instance: %w", err)
	}

	if p.DeviceId != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO devices
				(id, driver_instance_id, friendly_name, disabled, created_at, updated_at)
			VALUES (?, ?, '', 0, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			p.DeviceId, p.DriverInstanceId, now, now,
		); err != nil {
			return fmt.Errorf("ensure device: %w", err)
		}
	}

	var deviceID sql.NullString
	if p.DeviceId != "" {
		deviceID = sql.NullString{String: p.DeviceId, Valid: true}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO entities
			(id, device_id, driver_instance_id, entity_type, friendly_name, capabilities, disabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			device_id          = excluded.device_id,
			driver_instance_id = excluded.driver_instance_id,
			entity_type        = excluded.entity_type,
			friendly_name      = excluded.friendly_name,
			capabilities       = excluded.capabilities,
			disabled           = 0,
			updated_at         = excluded.updated_at`,
		e.Entity, deviceID, p.DriverInstanceId, p.EntityType, p.FriendlyName, caps, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert entity: %w", err)
	}
	return nil
}

func (r *Registry) applyEntityUnregistered(ctx context.Context, tx storage.Tx, e eventstore.Event) error {
	_, err := tx.ExecContext(ctx, `UPDATE entities SET disabled = 1, updated_at = ? WHERE id = ?`,
		e.Timestamp.UnixNano(), e.Entity)
	if err != nil {
		return fmt.Errorf("disable entity %s: %w", e.Entity, err)
	}
	return nil
}

func (r *Registry) applyDriverEvent(ctx context.Context, tx storage.Tx, e eventstore.Event, p *eventv1.DriverEvent) error {
	ts := e.Timestamp.UnixNano()
	switch p.Kind {
	case "started":
		_, err := tx.ExecContext(ctx, `
			INSERT INTO driver_instances
				(id, driver_name, display_name, transport, endpoint, config_hash, status, started_at, last_heartbeat, created_at)
			VALUES (?, ?, ?, 'local_subprocess', '', '', 'running', ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				status         = 'running',
				last_error     = NULL,
				started_at     = excluded.started_at,
				last_heartbeat = excluded.last_heartbeat`,
			p.DriverInstanceId, p.DriverInstanceId, p.DriverInstanceId, ts, ts, ts,
		)
		if err != nil {
			return fmt.Errorf("driver started %s: %w", p.DriverInstanceId, err)
		}
	case "stopped":
		if _, err := tx.ExecContext(ctx, `UPDATE driver_instances SET status = 'stopped' WHERE id = ?`, p.DriverInstanceId); err != nil {
			return fmt.Errorf("driver stopped %s: %w", p.DriverInstanceId, err)
		}
	case "failed":
		if _, err := tx.ExecContext(ctx, `UPDATE driver_instances SET status = 'failed', last_error = ? WHERE id = ?`,
			p.Detail, p.DriverInstanceId); err != nil {
			return fmt.Errorf("driver failed %s: %w", p.DriverInstanceId, err)
		}
	case "heartbeat":
		if _, err := tx.ExecContext(ctx, `UPDATE driver_instances SET last_heartbeat = ? WHERE id = ?`, ts, p.DriverInstanceId); err != nil {
			return fmt.Errorf("driver heartbeat %s: %w", p.DriverInstanceId, err)
		}
	}
	return nil
}
