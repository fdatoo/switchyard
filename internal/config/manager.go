package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
)

// CarportManager is the subset of carport.Host that config.Manager needs.
// For C4, the daemon passes a no-op implementation; dynamic carport management
// will be wired when carport.Host gains RegisterInstance/UnregisterInstance methods.
type CarportManager interface {
	RegisterInstance(ctx context.Context, id, driverName string, params []byte) error
	UnregisterInstance(ctx context.Context, id string) error
}

// eventStore is the subset of eventstore.Store that Manager needs.
type eventStore interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// Manager is the main entry point for config evaluation, validation, and application.
// It is safe for concurrent use.
type Manager struct {
	configDir  string
	ev         configEvaluator
	store      eventStore
	carportMgr CarportManager
	keyring    Keyring

	mu           sync.RWMutex
	current      *configpb.ConfigSnapshot
	appliedHooks []func(snap *configpb.ConfigSnapshot)
}

// OnApplied registers a callback that is invoked after each successful Apply.
// Callbacks fire synchronously in registration order after store.Append succeeds.
func (m *Manager) OnApplied(fn func(*configpb.ConfigSnapshot)) {
	m.mu.Lock()
	m.appliedHooks = append(m.appliedHooks, fn)
	m.mu.Unlock()
}

// NewManager creates a Manager that evaluates config at configDir/main.pkl.
func NewManager(ctx context.Context, configDir string, store eventStore, carportMgr CarportManager) (*Manager, error) {
	ev, err := newPklEvaluator(ctx)
	if err != nil {
		return nil, fmt.Errorf("init pkl evaluator: %w", err)
	}
	return &Manager{
		configDir:  configDir,
		ev:         ev,
		store:      store,
		carportMgr: carportMgr,
	}, nil
}

// Current returns the most-recently-applied ConfigSnapshot. Nil before first Apply.
func (m *Manager) Current() *configpb.ConfigSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Validate evaluates and cross-ref-validates config. Returns snapshot + diff with no side-effects.
func (m *Manager) Validate(ctx context.Context) (*configpb.ConfigSnapshot, *ConfigDiff, error) {
	snap, err := m.ev.Evaluate(ctx, m.configDir)
	if err != nil {
		return nil, nil, err
	}
	if errs := Compile(snap, nil); len(errs) != 0 {
		return nil, nil, &compileErrors{errs: errs}
	}
	m.mu.RLock()
	diff := Diff(m.current, snap)
	m.mu.RUnlock()
	return snap, diff, nil
}

// Apply runs Validate, resolves secrets, applies carport side-effects, and appends ConfigApplied.
// If dryRun is true, stops after diff — no secrets resolved, no events appended.
func (m *Manager) Apply(ctx context.Context, dryRun bool) error {
	snap, diff, err := m.Validate(ctx)
	if err != nil {
		return err
	}
	if dryRun {
		return nil
	}

	if err := ResolveSecrets(ctx, snap, m.keyring); err != nil {
		return fmt.Errorf("resolve secrets: %w", err)
	}

	for _, id := range diff.DriverInstancesRemoved {
		if err := m.carportMgr.UnregisterInstance(ctx, id); err != nil {
			return fmt.Errorf("unregister %q: %w", id, err)
		}
	}
	for _, id := range diff.DriverInstancesAdded {
		di := findInstance(snap, id)
		if err := m.carportMgr.RegisterInstance(ctx, di.GetId(), di.GetDriverName(), di.GetParams()); err != nil {
			return fmt.Errorf("register %q: %w", id, err)
		}
	}
	for _, id := range diff.DriverInstancesChanged {
		di := findInstance(snap, id)
		if err := m.carportMgr.UnregisterInstance(ctx, id); err != nil {
			return fmt.Errorf("unregister changed %q: %w", id, err)
		}
		if err := m.carportMgr.RegisterInstance(ctx, di.GetId(), di.GetDriverName(), di.GetParams()); err != nil {
			return fmt.Errorf("re-register changed %q: %w", id, err)
		}
	}

	m.mu.Lock()
	m.current = snap
	m.mu.Unlock()

	_, err = m.store.Append(ctx, eventstore.Event{
		Kind:      "config",
		Source:    "config.Manager",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_ConfigApplied{
			ConfigApplied: &eventv1.ConfigApplied{
				AppliedAtUnixMs:        snap.GetEvaluatedAtUnixMs(),
				DriverInstancesAdded:   int32(len(diff.DriverInstancesAdded)),
				DriverInstancesRemoved: int32(len(diff.DriverInstancesRemoved)),
				DriverInstancesChanged: int32(len(diff.DriverInstancesChanged)),
				AutomationsChanged:     int32(diff.AutomationsChanged),
			},
		}},
	})
	if err == nil {
		m.mu.RLock()
		hooks := make([]func(*configpb.ConfigSnapshot), len(m.appliedHooks))
		copy(hooks, m.appliedHooks)
		current := m.current
		m.mu.RUnlock()
		for _, h := range hooks {
			h(current)
		}
	}
	return err
}

func findInstance(snap *configpb.ConfigSnapshot, id string) *configpb.DriverInstanceConfig {
	for _, di := range snap.GetDriverInstances() {
		if di.GetId() == id {
			return di
		}
	}
	return nil
}

// compileErrors wraps multiple ValidationErrors for clean error rendering.
type compileErrors struct {
	errs []ValidationError
}

func (e *compileErrors) Error() string {
	if len(e.errs) == 1 {
		return e.errs[0].Error()
	}
	return fmt.Sprintf("%d validation errors (first: %s)", len(e.errs), e.errs[0].Error())
}

// Errors returns the individual ValidationErrors for CLI rendering.
func (e *compileErrors) Errors() []ValidationError { return e.errs }
