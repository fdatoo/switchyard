package carport

import (
	"context"
	"fmt"
	"strings"

	"github.com/fdatoo/switchyard/internal/registry"
)

// Router resolves an entity_id to its owning driver_instance_id by reading the
// registry projection (which itself is populated from the event log).
type Router struct {
	reg *registry.Registry
}

// NewRouter wraps a registry projection for lookups.
func NewRouter(reg *registry.Registry) *Router {
	return &Router{reg: reg}
}

// Resolve returns the driver_instance_id that currently owns the entity, or
// wraps ErrEntityUnknown if the entity isn't registered.
func (r *Router) Resolve(ctx context.Context, entityID string) (string, error) {
	e, err := r.reg.GetEntity(ctx, entityID)
	if err != nil {
		// GetEntity returns a fmt.Errorf("entity not found") on sql.ErrNoRows —
		// we can't errors.Is against it, so do a message check.
		if strings.Contains(err.Error(), "not found") {
			return "", fmt.Errorf("resolve %q: %w", entityID, ErrEntityUnknown)
		}
		return "", fmt.Errorf("resolve %q: %w", entityID, err)
	}
	if e.DriverInstanceID == "" {
		return "", fmt.Errorf("resolve %q: entity has no driver_instance_id: %w", entityID, ErrEntityUnknown)
	}
	return e.DriverInstanceID, nil
}
