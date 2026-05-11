package replay

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	replayv1 "github.com/fdatoo/switchyard/gen/switchyard/replay/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/replay/v1/replayv1connect"
)

// Compile-time assertion that Service implements the handler interface.
var _ replayv1connect.ReplayServiceHandler = (*Service)(nil)

// EventLookup provides event retrieval by ID and by sequence.
type EventLookup interface {
	EventByID(ctx context.Context, eventID string) (EntityEvent, bool, error)
	EventBySeq(ctx context.Context, seq uint64) (EntityEvent, bool, error)
}

// Service implements ReplayServiceHandler.
type Service struct {
	snaps   SnapshotStore
	events  EventReader
	byID    EventLookup
	// bySeq is also EventLookup — same interface
}

// NewService creates a ReplayService.
// store, reader, and lookup may be the same underlying object (typical in production).
func NewService(snaps SnapshotStore, reader EventReader, byID EventLookup, _ EventLookup) *Service {
	return &Service{
		snaps:  snaps,
		events: reader,
		byID:   byID,
	}
}

// LoadAtSeq reconstructs entity state at the given seq, diffing against seq-1.
func (s *Service) LoadAtSeq(
	ctx context.Context,
	req *connect.Request[replayv1.LoadAtSeqRequest],
) (*connect.Response[replayv1.LoadAtSeqResponse], error) {
	seq := req.Msg.Seq

	// Get snapshot and replay to seq.
	snap, err := nearestSnapshot(ctx, s.snaps, seq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	stateNow, err := replayForward(ctx, s.events, snap, seq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get state at seq-1 for diff.
	var statePrev EntityStateMap
	if seq > 0 {
		snapPrev, err := nearestSnapshot(ctx, s.snaps, seq-1)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		statePrev, err = replayForward(ctx, s.events, snapPrev, seq-1)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	diff := computeDiff(statePrev, stateNow)

	// Build proto entities.
	protoEntities := make([]*replayv1.EntityState, 0, len(stateNow))
	for entityID, fields := range stateNow {
		protoEntities = append(protoEntities, &replayv1.EntityState{
			EntityId: entityID,
			Fields:   fields,
		})
	}

	// Fetch event metadata for this seq.
	resp := &replayv1.LoadAtSeqResponse{
		Seq:      seq,
		Entities: protoEntities,
		Diff:     diff,
	}

	evt, found, err := s.byID.(interface {
		EventBySeq(context.Context, uint64) (EntityEvent, bool, error)
	}).EventBySeq(ctx, seq)
	if err == nil && found {
		resp.EventId = evt.EventID
		resp.Kind = evt.Kind
		resp.EntityId = evt.EntityID
		resp.Source = evt.Source
		resp.CausationId = evt.CausationID
		resp.CorrelationId = evt.CorrelationID
		resp.Emitter = evt.Emitter
		resp.SpanId = evt.SpanID
		resp.OccurredAt = timestamppb.New(evt.OccurredAt)
		resp.PayloadJson = evt.PayloadJSON
		resp.WhyInteresting = evt.WhyInteresting
	}

	return connect.NewResponse(resp), nil
}

// CausationChain walks the causation_id chain and returns events root-first.
func (s *Service) CausationChain(
	ctx context.Context,
	req *connect.Request[replayv1.CausationChainRequest],
) (*connect.Response[replayv1.CausationChainResponse], error) {
	const maxDepth = 256

	// Walk backwards from the requested event_id to find the root.
	var chain []EntityEvent
	currentID := req.Msg.EventId
	visited := make(map[string]bool)

	for i := 0; i < maxDepth; i++ {
		if currentID == "" {
			break
		}
		if visited[currentID] {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("causation cycle detected at %s", currentID))
		}
		visited[currentID] = true

		evt, found, err := s.byID.EventByID(ctx, currentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !found {
			break
		}
		chain = append(chain, evt)
		currentID = evt.CausationID
	}

	// Reverse to get root-first order.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	protoEvents := make([]*replayv1.ChainEvent, 0, len(chain))
	for _, e := range chain {
		protoEvents = append(protoEvents, entityEventToChainEvent(e))
	}

	return connect.NewResponse(&replayv1.CausationChainResponse{
		Events: protoEvents,
	}), nil
}

// Window returns event metadata for the given sequence range.
func (s *Service) Window(
	ctx context.Context,
	req *connect.Request[replayv1.WindowRequest],
) (*connect.Response[replayv1.WindowResponse], error) {
	events, err := s.events.EventsInRange(ctx, req.Msg.FromSeq, req.Msg.ToSeq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoEvents := make([]*replayv1.ChainEvent, 0, len(events))
	for _, e := range events {
		protoEvents = append(protoEvents, entityEventToChainEvent(e))
	}

	return connect.NewResponse(&replayv1.WindowResponse{
		Events: protoEvents,
	}), nil
}

// computeDiff returns the server-side StateDiff between prev and now state maps.
func computeDiff(prev, now EntityStateMap) *replayv1.StateDiff {
	diff := &replayv1.StateDiff{}
	// Check all entities in now.
	for entityID, nowFields := range now {
		prevFields := prev[entityID]
		var fieldDiffs []*replayv1.FieldDiff
		for field, nowVal := range nowFields {
			prevVal := prevFields[field]
			if nowVal != prevVal {
				fieldDiffs = append(fieldDiffs, &replayv1.FieldDiff{
					Field: field,
					Was:   prevVal,
					Now:   nowVal,
				})
			}
		}
		// Check deleted fields.
		for field, prevVal := range prevFields {
			if _, ok := nowFields[field]; !ok {
				fieldDiffs = append(fieldDiffs, &replayv1.FieldDiff{
					Field: field,
					Was:   prevVal,
					Now:   "",
				})
			}
		}
		if len(fieldDiffs) > 0 {
			diff.EntityDiffs = append(diff.EntityDiffs, &replayv1.EntityDiff{
				EntityId:   entityID,
				FieldDiffs: fieldDiffs,
			})
		}
	}
	return diff
}

func entityEventToChainEvent(e EntityEvent) *replayv1.ChainEvent {
	ce := &replayv1.ChainEvent{
		EventId:     e.EventID,
		Seq:         e.Seq,
		CausationId: e.CausationID,
		Kind:        e.Kind,
		EntityId:    e.EntityID,
	}
	if !e.OccurredAt.IsZero() {
		ce.OccurredAt = timestamppb.New(e.OccurredAt)
	}
	return ce
}
