package driver

import (
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
)

// EntitySpec describes a driver-owned entity.
type EntitySpec struct {
	EntityType   string // "light", "sensor", "switch", etc.
	FriendlyName string
	Capabilities []string // advertised in the manifest
	// InitialState is the entity's current state at registration time.
	// Optional — drivers that don't know initial state can leave it nil and
	// emit a StateChanged later. When set, it travels in the EntityRegistered
	// message and seeds the daemon's state cache before the Run stream opens,
	// so the very first state dump shows real values rather than empty.
	InitialState *entityv1.Attributes
}
