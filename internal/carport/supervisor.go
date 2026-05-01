package carport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/grpc/status"

	carportpb "github.com/fdatoo/switchyard/gen/switchyard/carport/v1alpha1"
	eventpb "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

// launchLifecycle starts a per-instance supervisor goroutine. It sets
// m.cancelLifecycle before returning so shutdownInstance can interrupt it.
func (h *Host) launchLifecycle(parent context.Context, m *managedInstance) {
	ctx, cancel := context.WithCancel(parent)
	m.cancelLifecycle = cancel
	m.done = make(chan struct{})
	go func() {
		defer close(m.done)
		h.runLifecycle(ctx, m)
	}()
}

// runLifecycle is the per-instance FSM loop: spawning → handshake → running →
// failed → backoff → (spawning | quarantined). Exits when ctx is cancelled or
// the instance is quarantined.
func (h *Host) runLifecycle(ctx context.Context, m *managedInstance) {
	for {
		if ctx.Err() != nil {
			return
		}

		m.mu.Lock()
		m.state = StateSpawning
		m.mu.Unlock()

		sock, proc, secret, err := h.spawn(ctx, m.cfg)
		if err != nil {
			h.emitDriverEvent(ctx, m, "spawn_error", err.Error())
			h.transitionToFailed(ctx, m)
			if !h.scheduleBackoff(ctx, m) {
				return
			}
			continue
		}

		m.mu.Lock()
		m.state = StateAwaitingHandshake
		m.mu.Unlock()
		h.emitDriverEvent(ctx, m, "spawned", fmt.Sprintf("%d", proc.Process.Pid))

		ic, handshakeResp, err := h.handshake(ctx, m.cfg, sock, secret)
		if err != nil {
			h.metrics.CarportHandshakesTotal.WithLabelValues(m.cfg.ID, "fail").Inc()
			if proc.Process != nil {
				_ = proc.Process.Kill()
			}
			_ = proc.Wait()
			h.emitDriverEvent(ctx, m, "handshake_failed", err.Error())
			h.transitionToFailed(ctx, m)
			if !h.scheduleBackoff(ctx, m) {
				return
			}
			continue
		}
		h.metrics.CarportHandshakesTotal.WithLabelValues(m.cfg.ID, "ok").Inc()

		m.mu.Lock()
		m.state = StateRunning
		m.conn = ic
		m.mu.Unlock()
		// NOTE: CarportDriverInstances reflects the last observed state per instance
		// rather than an exact per-state count across all instances. This is imprecise
		// when multiple instances share a state but avoids maintaining an atomic
		// reference count per state across concurrent lifecycle goroutines.
		h.metrics.CarportDriverInstances.WithLabelValues(StateRunning.String()).Set(1)

		manifestVersion := ""
		if handshakeResp.GetManifest() != nil {
			manifestVersion = handshakeResp.GetManifest().GetVersion()
		}
		h.emitDriverEvent(ctx, m, "started", manifestVersion)

		// Apply initial entity registrations from the handshake response.
		for _, er := range handshakeResp.GetInitialEntities() {
			if er.DriverInstanceId == "" {
				er.DriverInstanceId = m.cfg.ID
			}
			_, _ = h.store.Append(ctx, eventstore.Event{
				Timestamp: time.Now(),
				Kind:      "entity_registered",
				Entity:    er.GetDeviceId(),
				Source:    "driver:" + m.cfg.ID,
				Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityRegistered{
					EntityRegistered: er,
				}},
			})
		}

		// Wire ingest hook: translates DriverToHost messages into store events.
		ic.setIngestHook(func(msg *carportpb.DriverToHost) {
			kind := messageKindLabel(msg)
			h.metrics.CarportStreamMessagesTotal.WithLabelValues(m.cfg.ID, kind).Inc()
			if ingestErr := IngestMessage(ctx, h.store, m.cfg.ID, msg); ingestErr != nil {
				h.logger.Error("ingest failed", "instance_id", m.cfg.ID, "err", ingestErr)
				_ = ic.Close()
				return
			}
			h.metrics.CarportEventsIngestedTotal.WithLabelValues(m.cfg.ID, kind).Inc()
		})

		// Wire stream error hook: log only — the reader goroutine already called failAll.
		// Demote to debug when the host is shutting down; the cancellation IS the
		// "error" and there's nothing the operator should do about it.
		ic.setStreamErrorHook(func(streamErr error) {
			select {
			case <-h.stopped:
				h.logger.Debug("stream error during shutdown", "instance_id", m.cfg.ID, "err", streamErr)
			default:
				h.logger.Warn("stream error", "instance_id", m.cfg.ID, "err", streamErr)
			}
		})

		// Block until health fails or context is cancelled.
		healthy := h.runHealth(ctx, m, ic)
		_ = ic.Close()

		// Give the driver process a moment to exit cleanly after stream closure.
		doneCh := make(chan struct{})
		go func() {
			_ = proc.Wait()
			close(doneCh)
		}()
		select {
		case <-doneCh:
		case <-time.After(500 * time.Millisecond):
			if proc.Process != nil {
				_ = proc.Process.Kill()
			}
			<-doneCh
		}

		m.mu.Lock()
		m.conn = nil
		m.mu.Unlock()

		if ctx.Err() != nil {
			h.transitionToStopped(ctx, m, "daemon shutdown")
			return
		}

		cause := "stream closed"
		if !healthy {
			cause = "health failed"
		}
		h.emitDriverEvent(ctx, m, "failed", cause)
		h.transitionToFailed(ctx, m)
		if !h.scheduleBackoff(ctx, m) {
			return
		}
	}
}

// spawn forks the driver process and waits up to 1 s for its Unix socket to appear.
// Returns (socketPath, cmd, handshakeSecret, error).
func (h *Host) spawn(ctx context.Context, cfg Instance) (string, *exec.Cmd, string, error) {
	if err := os.MkdirAll(h.cfg.SocketDir, 0o750); err != nil {
		return "", nil, "", fmt.Errorf("mkdir socket dir: %w", err)
	}
	socketPath := filepath.Join(h.cfg.SocketDir, cfg.ID+".sock")
	_ = os.Remove(socketPath)

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, "", fmt.Errorf("rand secret: %w", err)
	}
	secret := hex.EncodeToString(buf)

	cmd := exec.CommandContext(ctx, cfg.Binary)
	cmd.Env = append(os.Environ(),
		"SWITCHYARD_CARPORT_SOCKET="+socketPath,
		"SWITCHYARD_CARPORT_SECRET="+secret,
		"SWITCHYARD_CARPORT_INSTANCE_ID="+cfg.ID,
		"SWITCHYARD_CARPORT_INSTANCE_CONFIG="+string(cfg.ConfigJSON),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", nil, "", fmt.Errorf("start driver process: %w", err)
	}

	// Poll for socket appearance: 50 × 20 ms = 1 s maximum.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, cmd, secret, nil
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return "", nil, "", ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	_ = cmd.Process.Kill()
	return "", nil, "", fmt.Errorf("socket %s never appeared after 1s", socketPath)
}

// handshake dials the driver socket, sends a HandshakeRequest and validates the
// response. Returns the open instanceConn on success; caller owns Close().
func (h *Host) handshake(ctx context.Context, cfg Instance, socketPath, secret string) (*instanceConn, *carportpb.HandshakeResponse, error) {
	dctx, cancel := context.WithTimeout(ctx, cfg.Lifecycle.HandshakeDeadline)
	defer cancel()

	ic, err := DialInstance(dctx, socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("dial instance: %w", err)
	}

	resp, err := ic.Handshake(dctx, &carportpb.HandshakeRequest{
		ProtocolVersion: "v1alpha1",
		InstanceId:      cfg.ID,
		HandshakeSecret: secret,
		InstanceConfig:  cfg.ConfigJSON,
	})
	if err != nil {
		_ = ic.Close()
		if st, ok := status.FromError(err); ok {
			return nil, nil, fmt.Errorf("handshake RPC: %s", st.Message())
		}
		return nil, nil, fmt.Errorf("handshake RPC: %w", err)
	}

	if resp.GetProtocolVersion() != "v1alpha1" {
		_ = ic.Close()
		return nil, nil, fmt.Errorf("protocol mismatch: want v1alpha1, got %q", resp.GetProtocolVersion())
	}

	return ic, resp, nil
}

// runHealth ticks the health probe on HealthProbeInterval until HealthFailuresToRestart
// consecutive failures occur (returns false) or ctx is done (returns true).
func (h *Host) runHealth(ctx context.Context, m *managedInstance, ic *instanceConn) bool {
	t := time.NewTicker(m.cfg.Lifecycle.HealthProbeInterval)
	defer t.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return true
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, m.cfg.Lifecycle.HealthProbeTimeout)
			start := time.Now()
			resp, err := ic.Health(pctx)
			h.metrics.CarportHealthProbeSeconds.WithLabelValues(m.cfg.ID).Observe(time.Since(start).Seconds())
			cancel()

			if err != nil || (resp != nil && !resp.Ok) {
				failures++
				h.logger.Warn("health probe failure",
					"instance_id", m.cfg.ID,
					"failures", failures,
					"max", m.cfg.Lifecycle.HealthFailuresToRestart,
				)
				if failures >= m.cfg.Lifecycle.HealthFailuresToRestart {
					return false
				}
			} else {
				failures = 0
			}
		}
	}
}

// transitionToFailed sets state to StateFailed without emitting an event
// (callers emit more specific events before calling this).
func (h *Host) transitionToFailed(_ context.Context, m *managedInstance) {
	m.mu.Lock()
	m.state = StateFailed
	m.mu.Unlock()
}

// transitionToStopped emits a "stopped" DriverEvent and sets state to StateStopped.
func (h *Host) transitionToStopped(ctx context.Context, m *managedInstance, reason string) {
	h.emitDriverEvent(ctx, m, "stopped", reason)
	m.mu.Lock()
	m.state = StateStopped
	m.mu.Unlock()
}

// scheduleBackoff records this restart attempt, checks the budget, computes
// exponential backoff, and sleeps. Returns true to retry, false to exit
// (quarantine reached or ctx cancelled).
func (h *Host) scheduleBackoff(ctx context.Context, m *managedInstance) bool {
	now := time.Now()

	m.mu.Lock()
	cutoff := now.Add(-m.cfg.Lifecycle.RestartBudgetWindow).UnixNano()
	fresh := make([]int64, 0, len(m.restartHistory))
	for _, ts := range m.restartHistory {
		if ts >= cutoff {
			fresh = append(fresh, ts)
		}
	}
	fresh = append(fresh, now.UnixNano())
	m.restartHistory = fresh
	used := len(m.restartHistory)
	maxBudget := m.cfg.Lifecycle.RestartBudgetMax
	initial := m.cfg.Lifecycle.RestartBackoffInitial
	maxBackoff := m.cfg.Lifecycle.RestartBackoffMax
	m.mu.Unlock()

	if used > maxBudget {
		h.metrics.CarportDriverRestartsTotal.WithLabelValues(m.cfg.ID, "quarantined").Inc()
		h.emitDriverEvent(ctx, m, "quarantined", "restart budget exhausted")
		m.mu.Lock()
		m.state = StateQuarantined
		m.mu.Unlock()
		return false
	}

	// Exponential backoff: initial * 2^(n-1), capped at max.
	backoff := initial
	for i := 1; i < used; i++ {
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
			break
		}
	}

	h.metrics.CarportDriverRestartsTotal.WithLabelValues(m.cfg.ID, "backoff_retry").Inc()
	h.emitDriverEvent(ctx, m, "backoff_scheduled", backoff.String())
	m.mu.Lock()
	m.state = StateBackoff
	m.mu.Unlock()

	select {
	case <-time.After(backoff):
		return true
	case <-ctx.Done():
		return false
	}
}

// shutdownInstance gracefully stops a managed instance:
//  1. Transitions state to StateStopping and emits the "stopping" event (if applicable).
//  2. Cancels the lifecycle goroutine context FIRST so it exits via transitionToStopped
//     rather than racing to emit a spurious "failed" event.
//  3. Sends the Shutdown RPC (bounded by ShutdownGrace) if a connection is open.
func (h *Host) shutdownInstance(ctx context.Context, m *managedInstance) {
	m.mu.Lock()
	prev := m.state
	m.state = StateStopping
	conn := m.conn
	m.mu.Unlock()

	// Skip the "stopping" event when the instance is already quiesced.
	if prev != StateQuarantined && prev != StateStopped {
		h.emitDriverEvent(ctx, m, "stopping", prev.String())
	}

	// Cancel lifecycle FIRST so the runLifecycle goroutine sees ctx.Done and exits
	// via transitionToStopped rather than overwriting StateStopping with StateFailed.
	if m.cancelLifecycle != nil {
		m.cancelLifecycle()
	}

	if conn != nil {
		sctx, cancel := context.WithTimeout(ctx, m.cfg.Lifecycle.ShutdownGrace)
		_, _ = conn.Shutdown(sctx, m.cfg.Lifecycle.ShutdownGrace.Milliseconds())
		cancel()
		_ = conn.Close()
	}

	// Wait for the lifecycle goroutine to actually exit so its terminal
	// event Appends complete before we return — otherwise the daemon's
	// final snapshot races us.
	if m.done != nil {
		select {
		case <-m.done:
		case <-ctx.Done():
		}
	}
}

// emitDriverEvent appends a driver_event to the event store and logs the transition.
func (h *Host) emitDriverEvent(ctx context.Context, m *managedInstance, kind, detail string) {
	// If the caller context is already cancelled (typical at daemon shutdown),
	// use a fresh background context with a short timeout so terminal events
	// like "stopped" still land in the log. Same pattern as dispatch.go's
	// CommandAck append.
	appendCtx := ctx //nolint:contextcheck // intentional: when ctx is cancelled (shutdown), we swap to a fresh ctx below so the terminal event still lands
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		appendCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	_, err := h.store.Append(appendCtx, eventstore.Event{
		Timestamp: time.Now(),
		Kind:      "driver_event",
		Source:    "carport:host",
		Payload: &eventpb.Payload{Kind: &eventpb.Payload_DriverEvent{
			DriverEvent: &eventpb.DriverEvent{
				DriverInstanceId: m.cfg.ID,
				Kind:             kind,
				Detail:           detail,
			},
		}},
	})
	if err != nil {
		h.logger.Error("emitDriverEvent append failed",
			"instance_id", m.cfg.ID, "kind", kind, "err", err)
	}
	h.logger.Info("driver transition",
		"instance_id", m.cfg.ID, "kind", kind, "detail", detail)
}

// messageKindLabel returns a stable string label for a DriverToHost message,
// used as the "kind" label on CarportStreamMessagesTotal and CarportEventsIngestedTotal.
func messageKindLabel(msg *carportpb.DriverToHost) string {
	switch msg.GetKind().(type) {
	case *carportpb.DriverToHost_Result:
		return "result"
	case *carportpb.DriverToHost_StateChanged:
		return "state_changed"
	case *carportpb.DriverToHost_EntityRegistered:
		return "entity_registered"
	case *carportpb.DriverToHost_EntityUnregistered:
		return "entity_unregistered"
	case *carportpb.DriverToHost_DriverEvent:
		return "driver_event"
	case *carportpb.DriverToHost_Pong:
		return "pong"
	default:
		return "unknown"
	}
}

// InstanceState returns the current FSM state for the given instance ID.
// Returns StateDeclared if the ID is not known.
func (h *Host) InstanceState(id string) State {
	h.mu.RLock()
	m, ok := h.instances[id]
	h.mu.RUnlock()
	if !ok {
		return StateDeclared
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}
