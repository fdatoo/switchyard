package config

import (
	"context"
	"sync"
	"time"
)

// ReloaderApplier abstracts the part of Manager.Apply that Reloader
// invokes. The real Manager satisfies this via an adapter (set up in
// the daemon wiring layer).
type ReloaderApplier interface {
	// Apply re-evaluates and applies config. `source` is a free-form
	// telemetry tag ("form" | "watcher" | "rpc") describing what
	// requested the reload.
	Apply(ctx context.Context, source string) error
}

// Reloader coalesces reload requests from three triggers (form save,
// watcher, RPC) into debounced Manager.Apply calls. It tracks the most
// recent Apply error so the Reload RPC can surface it to the user.
type Reloader struct {
	app      ReloaderApplier
	debounce time.Duration

	mu        sync.Mutex
	pending   []string
	lastErr   string

	signal chan struct{}
}

// NewReloader creates a Reloader with the given debounce window.
// Recommended: 250ms.
func NewReloader(app ReloaderApplier, debounce time.Duration) *Reloader {
	if debounce <= 0 {
		debounce = 250 * time.Millisecond
	}
	return &Reloader{
		app:      app,
		debounce: debounce,
		signal:   make(chan struct{}, 1),
	}
}

// Start spawns the dispatch goroutine. Cancelling ctx stops it.
func (r *Reloader) Start(ctx context.Context) {
	go r.loop(ctx)
}

// Trigger requests a reload. Multiple Trigger calls within `debounce` of
// each other coalesce into a single Apply.
func (r *Reloader) Trigger(source string) {
	r.mu.Lock()
	r.pending = append(r.pending, source)
	r.mu.Unlock()
	select {
	case r.signal <- struct{}{}:
	default:
	}
}

// LastError returns the most recent Apply error message, or "" if the
// most recent Apply succeeded (or none has run yet).
func (r *Reloader) LastError() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

func (r *Reloader) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.signal:
		}
		timer := time.NewTimer(r.debounce)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		select {
		case <-r.signal:
		default:
		}

		r.mu.Lock()
		sources := r.pending
		r.pending = nil
		r.mu.Unlock()

		source := "unknown"
		if len(sources) > 0 {
			source = sources[0]
		}

		err := r.app.Apply(ctx, source)
		r.mu.Lock()
		if err != nil {
			r.lastErr = err.Error()
		} else {
			r.lastErr = ""
		}
		r.mu.Unlock()
	}
}
