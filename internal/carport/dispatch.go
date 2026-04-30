package carport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	carportpb "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	eventpb "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/cause"
	"github.com/fdatoo/gohome/internal/eventstore"
)

const defaultDispatchTimeout = 30 * time.Second

// Dispatch sends a command to the driver instance owning entityID and waits
// for its CommandResult. On entry, if routing or instance-state checks fail, no
// event is appended and a pre-flight error is returned. Otherwise, CommandIssued
// is appended BEFORE the wire send, and CommandAck is appended AFTER — even on
// timeout, stream closure, or context cancel. INV-1: every CommandIssued has a
// matching CommandAck within the daemon's lifetime.
func (h *Host) Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*carportpb.CommandResult, error) {
	dispatchStart := time.Now()

	// 1. Routing.
	instanceID, err := h.router.Resolve(ctx, entityID)
	if err != nil {
		h.metrics.CarportCommandDispatchTotal.WithLabelValues("", "entity_unknown").Inc()
		return nil, err
	}

	// 2. Instance lookup + state check.
	h.mu.RLock()
	m, ok := h.instances[instanceID]
	h.mu.RUnlock()
	if !ok {
		h.metrics.CarportCommandDispatchTotal.WithLabelValues(instanceID, "instance_not_running").Inc()
		return nil, ErrInstanceNotRunning
	}
	m.mu.Lock()
	conn := m.conn
	state := m.state
	m.mu.Unlock()
	if state != StateRunning || conn == nil {
		h.metrics.CarportCommandDispatchTotal.WithLabelValues(instanceID, "instance_not_running").Inc()
		return nil, ErrInstanceNotRunning
	}

	// 3. Allocate command_id (host-assigned UUIDv4, echoed by driver).
	commandID := uuid.NewString()

	// 4. Append CommandIssued. Once this succeeds, INV-1 binds us to append a CommandAck.
	// If the caller attached automation lineage, record it as the Source for tracing.
	commandSource := "carport:host"
	if corr, ok := cause.FromCorrelation(ctx); ok && corr.AutomationID != "" {
		commandSource = "automation:" + corr.AutomationID + "#" + corr.CorrelationID
	}
	_, err = h.store.Append(ctx, eventstore.Event{
		Timestamp: time.Now(),
		Kind:      "command_issued",
		Entity:    entityID,
		Source:    commandSource,
		Payload: &eventpb.Payload{Kind: &eventpb.Payload_CommandIssued{
			CommandIssued: &eventpb.CommandIssued{
				Command:    capability,
				Parameters: args,
			},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("append command_issued: %w", err)
	}

	// 5. Compute effective deadline — use caller's if set, otherwise default.
	dctx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, defaultDispatchTimeout)
		defer cancel()
	}
	var deadlineMs int64
	if dl, ok := dctx.Deadline(); ok {
		deadlineMs = dl.UnixMilli()
	}

	// 6. Send + wait on the instance connection.
	cmd := &carportpb.Command{
		CommandId:      commandID,
		EntityId:       entityID,
		Capability:     capability,
		Args:           args,
		DeadlineUnixMs: deadlineMs,
	}
	h.metrics.CarportPendingCommands.WithLabelValues(instanceID).Inc()
	result, sendErr := conn.SendCommand(dctx, cmd)
	h.metrics.CarportPendingCommands.WithLabelValues(instanceID).Dec()

	// 7. INV-1: append CommandAck regardless of sendErr.
	ack := &eventpb.CommandAck{}
	mappedErr := mapSendError(sendErr)
	switch {
	case sendErr == nil && result != nil:
		ack.Success = result.Ok
		ack.ErrorMessage = result.ErrorMessage
	case errors.Is(mappedErr, ErrDispatchTimeout):
		ack.Success = false
		ack.ErrorMessage = "dispatch timeout"
	case errors.Is(mappedErr, ErrContextCanceled):
		ack.Success = false
		ack.ErrorMessage = "context canceled"
	case errors.Is(mappedErr, ErrStreamClosed):
		ack.Success = false
		ack.ErrorMessage = "driver stream closed"
	default:
		ack.Success = false
		if sendErr != nil {
			ack.ErrorMessage = sendErr.Error()
		}
	}
	// Append CommandAck with a fresh background ctx so a cancelled caller ctx
	// doesn't cause us to silently skip the Ack append (INV-1).
	ackCtx, ackCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ackCancel()
	if _, ackErr := h.store.Append(ackCtx, eventstore.Event{ //nolint:contextcheck
		Timestamp: time.Now(),
		Kind:      "command_ack",
		Entity:    entityID,
		Source:    "carport:host",
		Payload:   &eventpb.Payload{Kind: &eventpb.Payload_CommandAck{CommandAck: ack}},
	}); ackErr != nil {
		h.logger.Error("append command_ack", "entity", entityID, "err", ackErr)
	}

	// Emit dispatch outcome metrics.
	resultLabel := dispatchResultLabel(sendErr, mappedErr)
	h.metrics.CarportCommandDispatchTotal.WithLabelValues(instanceID, resultLabel).Inc()
	h.metrics.CarportCommandDispatchSeconds.WithLabelValues(instanceID, capability).Observe(time.Since(dispatchStart).Seconds())

	if sendErr != nil {
		return nil, mappedErr
	}
	return result, nil
}

// dispatchResultLabel maps a sendErr / mappedErr pair to a stable label string
// for CarportCommandDispatchTotal.
func dispatchResultLabel(sendErr, mappedErr error) string {
	if sendErr == nil {
		return "ok"
	}
	switch {
	case errors.Is(mappedErr, ErrDispatchTimeout):
		return "timeout"
	case errors.Is(mappedErr, ErrStreamClosed):
		return "stream_closed"
	case errors.Is(mappedErr, ErrContextCanceled):
		return "context_canceled"
	case errors.Is(mappedErr, ErrInstanceNotRunning):
		return "instance_not_running"
	case errors.Is(mappedErr, ErrEntityUnknown):
		return "entity_unknown"
	default:
		return "internal"
	}
}

// mapSendError translates raw transport/context errors into carport sentinel errors.
func mapSendError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return ErrContextCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrDispatchTimeout
	case errors.Is(err, ErrStreamClosed):
		return ErrStreamClosed
	case errors.Is(err, ErrDispatchTimeout):
		return ErrDispatchTimeout
	default:
		return err
	}
}

// InjectRunningInstanceForTests is an internal testing seam. It dials an
// already-serving in-process driver via its UDS path and registers it as a
// running managedInstance under instanceID, bypassing supervisor spawn/handshake.
// Callers own the backing server's lifecycle (typically via t.Cleanup).
func InjectRunningInstanceForTests(h *Host, instanceID, socketPath string) error {
	ic, err := DialInstance(context.Background(), socketPath)
	if err != nil {
		return err
	}
	m := &managedInstance{
		cfg: Instance{
			ID: instanceID,
			Lifecycle: LifecycleConfig{
				HandshakeDeadline: 5 * time.Second,
				ShutdownGrace:     1 * time.Second,
			},
		},
		state: StateRunning,
		conn:  ic,
	}
	h.mu.Lock()
	h.instances[instanceID] = m
	h.mu.Unlock()
	return nil
}
