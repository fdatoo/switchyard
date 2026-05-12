package carport

import (
	"context"
	"fmt"
)

// RestartInstance forces a driver instance back into the spawning loop,
// resetting its restart-budget history. Works on quarantined, running, or
// failed instances. Returns an error if the instance id is unknown.
func (h *Host) RestartInstance(ctx context.Context, id string) error {
	h.mu.RLock()
	m, ok := h.instances[id]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown instance %q", id)
	}

	// Tear down any existing live connection + lifecycle goroutine.
	h.shutdownInstance(ctx, m)

	// Reset budget history and state; emit the manual-restart event.
	m.mu.Lock()
	m.restartHistory = nil
	m.conn = nil
	m.state = StateDeclared
	m.mu.Unlock()

	h.emitDriverEvent(ctx, m, "restart_manual", "operator")

	// Relaunch supervisor goroutine on the host's long-lived context. We
	// cannot use the caller's ctx: when this RPC handler returns, connect-go
	// cancels the request ctx, which would propagate into the just-spawned
	// lifecycle goroutine and kill the driver almost immediately (the
	// backoff sleep cancels too, so it never recovers). RegisterInstance has
	// the same nolint for the same reason.
	h.launchLifecycle(h.ctx, m) //nolint:contextcheck // lifecycle goroutine must outlive the caller's context
	return nil
}
