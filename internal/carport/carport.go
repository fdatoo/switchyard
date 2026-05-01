// Package carport hosts the driver-supervisor subsystem: drivers.toml
// configuration, per-instance subprocess lifecycle, command dispatch, and
// event ingest from drivers over the Carport gRPC protocol (v1alpha1).
//
// See docs/superpowers/specs/2026-04-21-c2-carport-protocol-design.md.
package carport

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/registry"
)

// HostConfig is the daemon-level configuration handed to New.
type HostConfig struct {
	// DriversTOMLPath is the absolute path to drivers.toml. Empty or missing
	// file → zero configured instances (host is a no-op but still starts).
	DriversTOMLPath string

	// SocketDir is where per-instance UDS files are created. Defaults to
	// <data_dir>/carport/ in the daemon wiring (T18); required here if
	// any instance is to be spawned.
	SocketDir string
}

// Host is the public face of the carport subsystem. Instances are spawned
// and supervised on Start; Stop shuts them down gracefully.
type Host struct {
	cfg     HostConfig
	cfgData *Config

	db      *sql.DB
	store   *eventstore.Store
	router  *Router
	logger  *slog.Logger
	metrics *observability.Metrics

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
// Passing an empty DriversTOMLPath OR a path that does not exist yields a
// Host with zero instances; Start is then a no-op.
func New(cfg HostConfig, db *sql.DB, store *eventstore.Store, reg *registry.Registry, logger *slog.Logger, metrics *observability.Metrics) (*Host, error) {
	cfgData := &Config{}
	if cfg.DriversTOMLPath != "" {
		loaded, err := LoadConfig(cfg.DriversTOMLPath)
		if err != nil {
			return nil, err
		}
		cfgData = loaded
	}
	return &Host{
		cfg:       cfg,
		cfgData:   cfgData,
		db:        db,
		store:     store,
		router:    NewRouter(reg),
		logger:    logger.With("subsystem", "carport"),
		metrics:   metrics,
		instances: map[string]*managedInstance{},
		stopped:   make(chan struct{}),
	}, nil
}

// Start launches lifecycle goroutines for each enabled instance. Non-blocking.
// After Start returns, supervision goroutines are running; errors during
// spawn/handshake are reported through DriverEvents in the event log, not
// returned from this call.
func (h *Host) Start(ctx context.Context) error {
	for _, inst := range h.cfgData.Instances {
		if !inst.Enabled {
			continue
		}
		m := &managedInstance{cfg: inst, state: StateDeclared}
		h.mu.Lock()
		h.instances[inst.ID] = m
		h.mu.Unlock()
		h.launchLifecycle(ctx, m)
	}
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

// launchLifecycle and shutdownInstance are implemented in supervisor.go.
