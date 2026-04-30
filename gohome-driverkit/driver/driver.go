package driver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	carportv1alpha1 "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"

	"github.com/fdatoo/gohome-driverkit/protocol"
)

// Sentinel errors returned by Driver methods.
var (
	ErrEntityAlreadyRegistered = errors.New("entity already registered")
	ErrEntityUnknown           = errors.New("entity unknown")
	ErrNotConnected            = errors.New("no active run stream")
)

// CapabilityHandler handles a single capability invocation.
// The returned *entityv1.Attributes becomes the entity's tracked state and is
// emitted as a StateChanged event before the CommandResult is sent. Return
// (nil, nil) to acknowledge success without updating state.
type CapabilityHandler func(ctx context.Context, entityID string, args map[string]string) (*entityv1.Attributes, error)

type entityEntry struct {
	spec     EntitySpec
	attrs    *entityv1.Attributes
	handlers map[string]CapabilityHandler
}

// Driver is the high-level SDK entry point for writing Carport drivers.
type Driver struct {
	name    string
	version string

	mu       sync.RWMutex
	entities map[string]*entityEntry

	emitMu  sync.RWMutex
	emitter protocol.Emitter
}

// New creates a Driver with the given name and version.
func New(name, version string) *Driver {
	return &Driver{
		name:     name,
		version:  version,
		entities: make(map[string]*entityEntry),
	}
}

// AddEntity registers an entity. Call before Run.
// entityID format: "<type>.<name>" e.g. "light.kitchen".
func (d *Driver) AddEntity(entityID string, spec EntitySpec) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.entities[entityID]; ok {
		return fmt.Errorf("%w: %s", ErrEntityAlreadyRegistered, entityID)
	}
	d.entities[entityID] = &entityEntry{
		spec:     spec,
		attrs:    spec.InitialState,
		handlers: make(map[string]CapabilityHandler),
	}
	return nil
}

// OnCapability registers a handler for a specific entity+capability pair.
// Panics if entityID was not registered via AddEntity.
func (d *Driver) OnCapability(entityID, capability string, h CapabilityHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entities[entityID]
	if !ok {
		panic(fmt.Sprintf("driver.OnCapability: entity %q not registered via AddEntity", entityID))
	}
	e.handlers[capability] = h
}

// EmitState updates tracked state for entityID and sends a StateChanged event
// on the current Run stream. Safe to call from any goroutine.
// Returns ErrNotConnected if no stream is active.
func (d *Driver) EmitState(entityID string, attrs *entityv1.Attributes) error {
	d.mu.Lock()
	e, ok := d.entities[entityID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrEntityUnknown, entityID)
	}
	e.attrs = attrs
	d.mu.Unlock()

	d.emitMu.RLock()
	emit := d.emitter
	d.emitMu.RUnlock()
	if emit == nil {
		return ErrNotConnected
	}
	return emit.Send(&carportv1alpha1.DriverToHost{
		Kind: &carportv1alpha1.DriverToHost_StateChanged{
			StateChanged: &eventv1.StateChanged{EntityId: entityID, Attributes: attrs},
		},
	})
}

// Run reads Conn from env vars and calls RunConn in a reconnect loop until ctx
// is cancelled.
func (d *Driver) Run(ctx context.Context) error {
	conn, err := protocol.FromEnv()
	if err != nil {
		return err
	}
	return d.RunConn(ctx, conn)
}

// RunConn serves in a reconnect loop using the provided Conn until ctx is
// cancelled. Exported for testability — drivertest.New uses this directly
// to avoid env var manipulation.
//
// Backoff: 1s initial, 30s max, exponential. Resets to 1s after a session
// longer than 5s (distinguishes transient failures from crash loops).
func (d *Driver) RunConn(ctx context.Context, conn *protocol.Conn) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		start := time.Now()
		if err := conn.Serve(ctx, d); err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Since(start) > 5*time.Second {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff = min(backoff*2, 30*time.Second)
		}
	}
}

// --- protocol.Handler implementation ---

// OnHandshake implements protocol.Handler. Returns the manifest and initial
// entity list using the current tracked state for each entity's capabilities.
func (d *Driver) OnHandshake(_ []byte) (*carportv1alpha1.DriverManifest, []*eventv1.EntityRegistered, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	seen := make(map[string]bool)
	var caps []string
	for _, e := range d.entities {
		for _, c := range e.spec.Capabilities {
			if !seen[c] {
				caps = append(caps, c)
				seen[c] = true
			}
		}
	}

	sort.Strings(caps)

	manifest := &carportv1alpha1.DriverManifest{
		Name:                  d.name,
		Version:               d.version,
		ProtocolVersion:       "v1alpha1",
		SupportedCapabilities: caps,
	}

	var entities []*eventv1.EntityRegistered
	for entityID, e := range d.entities {
		// Capabilities carries the entity's *attributes* (the daemon's state
		// cache seeds itself from this on EntityRegistered). Use the tracked
		// attrs if present, else an empty Attributes so the daemon sees a
		// well-typed entity.
		caps := e.attrs
		if caps == nil {
			caps = &entityv1.Attributes{}
		}
		entities = append(entities, &eventv1.EntityRegistered{
			DeviceId:     entityID,
			EntityType:   e.spec.EntityType,
			FriendlyName: e.spec.FriendlyName,
			Capabilities: caps,
		})
	}

	sort.SliceStable(entities, func(i, j int) bool {
		return entities[i].DeviceId < entities[j].DeviceId
	})

	return manifest, entities, nil
}

// OnRunStart implements protocol.Handler. Stores the emitter and asserts
// current state for every entity with tracked attributes by emitting a
// StateChanged. The daemon's state cache treats EntityRegistered as
// register-once (capabilities), so re-registrations against an existing
// event log are no-ops in state. StateChanged is the durable truth channel
// — emitting on every Run start means a fresh daemon and a daemon
// replaying old events both converge to the driver's current view.
func (d *Driver) OnRunStart(_ context.Context, emit protocol.Emitter) {
	d.emitMu.Lock()
	d.emitter = emit
	d.emitMu.Unlock()

	d.mu.RLock()
	type pending struct {
		id    string
		attrs *entityv1.Attributes
	}
	out := make([]pending, 0, len(d.entities))
	for id, e := range d.entities {
		if e.attrs != nil {
			out = append(out, pending{id: id, attrs: e.attrs})
		}
	}
	d.mu.RUnlock()

	for _, p := range out {
		_ = emit.Send(&carportv1alpha1.DriverToHost{
			Kind: &carportv1alpha1.DriverToHost_StateChanged{
				StateChanged: &eventv1.StateChanged{EntityId: p.id, Attributes: p.attrs},
			},
		})
	}
}

// OnCommand implements protocol.Handler. Routes to the registered CapabilityHandler.
// StateChanged is sent before CommandResult so that drivertest.AssertState works
// immediately after SendCommand returns (StateChanged arrives before CommandResult
// on the stream; the harness reader processes it first).
func (d *Driver) OnCommand(ctx context.Context, cmd *carportv1alpha1.Command, emit protocol.Emitter) (*carportv1alpha1.CommandResult, error) {
	entityID := cmd.GetEntityId()

	d.mu.RLock()
	e, ok := d.entities[entityID]
	var handler CapabilityHandler
	if ok {
		handler = e.handlers[cmd.GetCapability()]
	}
	d.mu.RUnlock()

	if !ok {
		return &carportv1alpha1.CommandResult{
			CommandId:    cmd.GetCommandId(),
			Ok:           false,
			Code:         carportv1alpha1.CarportErrorCode_CARPORT_UNSUPPORTED_CAPABILITY,
			ErrorMessage: fmt.Sprintf("entity %q unknown", entityID),
		}, nil
	}
	if handler == nil {
		return &carportv1alpha1.CommandResult{
			CommandId:    cmd.GetCommandId(),
			Ok:           false,
			Code:         carportv1alpha1.CarportErrorCode_CARPORT_UNSUPPORTED_CAPABILITY,
			ErrorMessage: fmt.Sprintf("capability %q not registered for %q", cmd.GetCapability(), entityID),
		}, nil
	}

	attrs, err := handler(ctx, entityID, cmd.GetArgs())
	if err != nil {
		return &carportv1alpha1.CommandResult{
			CommandId:    cmd.GetCommandId(),
			Ok:           false,
			Code:         carportv1alpha1.CarportErrorCode_CARPORT_INTERNAL,
			ErrorMessage: err.Error(),
		}, nil
	}

	if attrs != nil {
		// Entity is guaranteed to exist: entries are never removed and we already
		// found it under RLock above.
		d.mu.Lock()
		d.entities[entityID].attrs = attrs
		d.mu.Unlock()

		// Send StateChanged before CommandResult so drivertest.AssertState sees
		// the new state when SendCommand returns. The send error is intentionally
		// dropped: a broken stream will also fail the CommandResult send below,
		// which tears the stream down and triggers the reconnect loop.
		_ = emit.Send(&carportv1alpha1.DriverToHost{
			Kind: &carportv1alpha1.DriverToHost_StateChanged{
				StateChanged: &eventv1.StateChanged{EntityId: entityID, Attributes: attrs},
			},
		})
	}

	return &carportv1alpha1.CommandResult{
		CommandId: cmd.GetCommandId(),
		Ok:        true,
	}, nil
}

// OnShutdown implements protocol.Handler. Clears the emitter.
func (d *Driver) OnShutdown(_ context.Context) error {
	d.emitMu.Lock()
	d.emitter = nil
	d.emitMu.Unlock()
	return nil
}
