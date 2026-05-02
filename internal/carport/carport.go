// Package carport hosts the driver-supervisor subsystem: per-instance subprocess
// lifecycle, command dispatch, and event ingest from drivers over the Carport
// gRPC protocol (v1alpha1). Driver instances are declared via main.pkl.
//
// See docs/superpowers/specs/2026-04-21-c2-carport-protocol-design.md.
package carport

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/registry"
)

// HostConfig is the daemon-level configuration handed to New.
type HostConfig struct {
	// SocketDir is where per-instance UDS files are created.
	SocketDir string
}

// Host is the public face of the carport subsystem. Instances are spawned
// and supervised on Start; Stop shuts them down gracefully.
type Host struct {
	cfg HostConfig

	db      *sql.DB
	store   *eventstore.Store
	router  *Router
	logger  *slog.Logger
	metrics *observability.Metrics

	ctx context.Context // root context for lifecycle goroutines; set by Start

	mu        sync.RWMutex
	instances map[string]*managedInstance // keyed by Instance.ID

	stopOnce sync.Once
	stopped  chan struct{}
}

// managedInstance bundles parsed Instance config + live FSM state + active
// *instanceConn (non-nil only when state == StateRunning).
//
// Concurrency: mu guards state/conn/restartHistory. cancelLifecycle is set once
// before the lifecycle goroutine starts and is read-only thereafter.
type managedInstance struct {
	cfg   Instance
	state State

	conn *instanceConn

	// restart bookkeeping — entries are Unix nanos of crash/restart moments
	// within the current RestartBudgetWindow.
	restartHistory []int64

	// cancelLifecycle cancels the per-instance lifecycle goroutine. Set by
	// launchLifecycle in supervisor.go (T11).
	cancelLifecycle context.CancelFunc
	// done is closed when the lifecycle goroutine actually exits. Lets
	// shutdownInstance block until any in-flight Appends (including the
	// terminal "stopped" event) have completed before returning, so the
	// daemon's final snapshot doesn't race the supervisor's writes.
	done chan struct{}

	mu sync.Mutex
}

// New constructs a Host. The host is inert until Start is called.
func New(cfg HostConfig, db *sql.DB, store *eventstore.Store, reg *registry.Registry, logger *slog.Logger, metrics *observability.Metrics) (*Host, error) {
	return &Host{
		cfg:       cfg,
		db:        db,
		store:     store,
		router:    NewRouter(reg),
		logger:    logger.With("subsystem", "carport"),
		metrics:   metrics,
		instances: map[string]*managedInstance{},
		stopped:   make(chan struct{}),
	}, nil
}

// Start initialises the host's root context. Non-blocking.
// Driver instances are added dynamically via RegisterInstance (driven by
// main.pkl evaluation in the config manager).
func (h *Host) Start(ctx context.Context) error {
	h.ctx = ctx
	return nil
}

// Stop signals every lifecycle goroutine to shut its instance down and waits
// (bounded by each instance's Lifecycle.ShutdownGrace) for a clean stop.
// Idempotent — safe to call multiple times.
func (h *Host) Stop(ctx context.Context) {
	h.stopOnce.Do(func() {
		close(h.stopped)
		h.mu.RLock()
		targets := make([]*managedInstance, 0, len(h.instances))
		for _, m := range h.instances {
			targets = append(targets, m)
		}
		h.mu.RUnlock()
		for _, m := range targets {
			h.shutdownInstance(ctx, m)
		}
	})
}

// RegisterInstance adds a new driver instance and begins its lifecycle goroutine.
// Implements config.CarportManager.
//
// Returns an error if an instance with that ID is already registered, if the host
// has not been started, or if the host has been stopped.
//
// driverName is currently unused at this layer (the supervisor identifies
// instances by ID and uses Binary for spawn) but is part of the interface so
// future work — e.g. event-source attribution, runtime manifest cross-checks —
// has the value without touching the call signature.
func (h *Host) RegisterInstance(_ context.Context, id, _, binary string, params []byte, enabled bool, lc LifecycleConfig) error {
	if h.ctx == nil {
		return fmt.Errorf("carport host not started")
	}
	select {
	case <-h.stopped:
		return fmt.Errorf("carport host is stopped")
	default:
	}
	h.mu.Lock()
	if _, exists := h.instances[id]; exists {
		h.mu.Unlock()
		return fmt.Errorf("instance %q already registered", id)
	}
	inst := Instance{
		ID:         id,
		Binary:     binary,
		Enabled:    enabled,
		ConfigJSON: params,
		Lifecycle:  lc,
	}
	m := &managedInstance{cfg: inst, state: StateDeclared}
	h.instances[id] = m
	h.mu.Unlock()
	h.launchLifecycle(h.ctx, m) //nolint:contextcheck // lifecycle goroutine must outlive the caller's context
	return nil
}

// RegisterInstanceWithLifecycle is a thin convenience wrapper preserved for
// existing test code. New callers should use RegisterInstance directly.
func (h *Host) RegisterInstanceWithLifecycle(ctx context.Context, id, driverName, binary string, params []byte, lc LifecycleConfig) error {
	return h.RegisterInstance(ctx, id, driverName, binary, params, true, lc)
}

// UnregisterInstance stops and removes a driver instance by ID.
// Returns an error if the instance is not found.
func (h *Host) UnregisterInstance(_ context.Context, id string) error {
	h.mu.Lock()
	m, exists := h.instances[id]
	if !exists {
		h.mu.Unlock()
		return fmt.Errorf("instance %q not found", id)
	}
	delete(h.instances, id)
	h.mu.Unlock()
	// Use Background context so shutdown isn't cut short by the caller's deadline.
	// Shutdown duration is bounded by m.cfg.Lifecycle.ShutdownGrace.
	h.shutdownInstance(context.Background(), m) //nolint:contextcheck
	return nil
}

// launchLifecycle and shutdownInstance are implemented in supervisor.go.
