package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

// CarportManager is the subset of carport.Host that config.Manager needs.
// RegisterInstance receives the fully-resolved binary path (looked up in the
// driver registry by Manager.Apply), the per-instance enabled flag, and the
// effective lifecycle config (defaults ← manifest ← per-instance override).
type CarportManager interface {
	RegisterInstance(ctx context.Context, id, driverName, binary string, params []byte, enabled bool, lifecycle carport.LifecycleConfig) error
	UnregisterInstance(ctx context.Context, id string) error
}

// eventStore is the subset of eventstore.Store that Manager needs.
type eventStore interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// Manager is the main entry point for config evaluation, validation, and application.
// It is safe for concurrent use.
type Manager struct {
	configDir   string
	driversRoot string
	ev          configEvaluator
	registry    *DriverRegistry
	store       eventStore
	carportMgr  CarportManager
	keyring     Keyring

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
// driversRoot is the directory the driver: URI scheme reader resolves against;
// pass an empty string only in test setups that won't import any driver: modules.
func NewManager(ctx context.Context, configDir, driversRoot string, store eventStore, carportMgr CarportManager) (*Manager, error) {
	ev, err := newPklEvaluator(ctx, driversRoot)
	if err != nil {
		return nil, fmt.Errorf("init pkl evaluator: %w", err)
	}
	registry, err := NewDriverRegistry(ctx, driversRoot)
	if err != nil {
		return nil, fmt.Errorf("scan drivers root %s: %w", driversRoot, err)
	}
	return &Manager{
		configDir:   configDir,
		driversRoot: driversRoot,
		ev:          ev,
		registry:    registry,
		store:       store,
		carportMgr:  carportMgr,
	}, nil
}

// registerInstance resolves the binary path and lifecycle for one instance
// (looking up the driver registry and merging defaults ← manifest ← override)
// and forwards to the carport. Skips registration if enabled=false.
func (m *Manager) registerInstance(ctx context.Context, di *configpb.DriverInstanceConfig) error {
	entry, ok := m.registry.Lookup(di.GetDriverName())
	if !ok {
		return fmt.Errorf("driver %q not installed at %s", di.GetDriverName(), m.driversRoot)
	}
	opts, err := parseInstanceOptions(di.GetParams())
	if err != nil {
		return fmt.Errorf("instance %q: %w", di.GetId(), err)
	}
	if !opts.Enabled {
		return nil
	}
	lifecycle := MergeLifecycle(entry.LifecycleDefaults, opts.Override)
	return m.carportMgr.RegisterInstance(ctx, di.GetId(), di.GetDriverName(), entry.BinaryPath, di.GetParams(), opts.Enabled, lifecycle)
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
		if err := m.registerInstance(ctx, di); err != nil {
			return fmt.Errorf("register %q: %w", id, err)
		}
	}
	for _, id := range diff.DriverInstancesChanged {
		di := findInstance(snap, id)
		if err := m.carportMgr.UnregisterInstance(ctx, id); err != nil {
			return fmt.Errorf("unregister changed %q: %w", id, err)
		}
		if err := m.registerInstance(ctx, di); err != nil {
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
