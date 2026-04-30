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

	// Relaunch supervisor goroutine with a fresh context derived from the
	// caller. If the caller's ctx is the daemon's shutdown ctx, this goroutine
	// will exit quickly when ctx is done — that's correct.
	h.launchLifecycle(ctx, m)
	return nil
}
