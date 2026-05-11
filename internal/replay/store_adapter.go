package replay

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/encoding/protojson"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

// StoreAdapter adapts *eventstore.Store to the SnapshotStore, EventReader,
// and EventLookup interfaces required by Service.
//
// Event IDs are the string representation of the event position (matching
// the convention used by ActivityService).
// Causation walks via CausePosition (not string ID).
type StoreAdapter struct {
	store *eventstore.Store
}

// NewStoreAdapter wraps an *eventstore.Store for use with ReplayService.
func NewStoreAdapter(store *eventstore.Store) *StoreAdapter {
	return &StoreAdapter{store: store}
}

// SnapshotBefore returns a Snapshot whose Seq ≤ seq.
// Since the event-store snapshot mechanism is per-projector and not directly
// queryable for entity-state maps, we implement this by returning a zero
// snapshot and letting replayForward replay all events from the beginning.
// This is correct but potentially slow for large stores; production can
// optimise later via a dedicated state-projector.
func (a *StoreAdapter) SnapshotBefore(_ context.Context, _ uint64) (Snapshot, error) {
	return Snapshot{Seq: 0, Entities: make(EntityStateMap)}, nil
}

// EventsInRange returns EntityEvents for positions in (fromSeq, toSeq].
func (a *StoreAdapter) EventsInRange(ctx context.Context, fromSeq, toSeq uint64) ([]EntityEvent, error) {
	raw, err := a.store.Query(ctx, eventstore.QueryOptions{
		FromPosition: fromSeq,
		ToPosition:   toSeq,
		Limit:        10000,
	})
	if err != nil {
		return nil, err
	}
	out := make([]EntityEvent, 0, len(raw))
	for _, e := range raw {
		out = append(out, storeEventToEntityEvent(e))
	}
	return out, nil
}

// EventByID parses the event_id as a position and fetches the event.
func (a *StoreAdapter) EventByID(ctx context.Context, eventID string) (EntityEvent, bool, error) {
	pos, err := strconv.ParseUint(eventID, 10, 64)
	if err != nil {
		return EntityEvent{}, false, nil // non-numeric ID = not found
	}
	return a.EventBySeq(ctx, pos)
}

// EventBySeq fetches the event at exactly the given position.
func (a *StoreAdapter) EventBySeq(ctx context.Context, seq uint64) (EntityEvent, bool, error) {
	if seq == 0 {
		return EntityEvent{}, false, nil
	}
	raw, err := a.store.Query(ctx, eventstore.QueryOptions{
		FromPosition: seq - 1,
		ToPosition:   seq,
		Limit:        1,
	})
	if err != nil {
		return EntityEvent{}, false, err
	}
	if len(raw) == 0 {
		return EntityEvent{}, false, nil
	}
	return storeEventToEntityEvent(raw[0]), true, nil
}

var protoJSONOpts = protojson.MarshalOptions{EmitUnpopulated: false}

func storeEventToEntityEvent(e eventstore.Event) EntityEvent {
	// Extract entity state fields from StateChanged payloads.
	fields := extractFields(e.Payload)

	causationID := ""
	if e.CausePosition > 0 {
		causationID = fmt.Sprintf("%d", e.CausePosition)
	}

	payloadJSON := ""
	if e.Payload != nil {
		if b, err := protoJSONOpts.Marshal(e.Payload); err == nil {
			payloadJSON = string(b)
		}
	}

	return EntityEvent{
		Seq:           e.Position,
		EntityID:      e.Entity,
		Fields:        fields,
		EventID:       fmt.Sprintf("%d", e.Position),
		Kind:          e.Kind,
		Source:        e.Source,
		CausationID:   causationID,
		CorrelationID: e.CorrelationID.String(),
		OccurredAt:    e.Timestamp,
		PayloadJSON:   payloadJSON,
	}
}

// extractFields returns a string-keyed field map from a StateChanged payload.
// For other payload types, returns an empty map (state is not updated).
func extractFields(p *eventv1.Payload) map[string]string {
	if p == nil {
		return map[string]string{}
	}
	sc, ok := p.Kind.(*eventv1.Payload_StateChanged)
	if !ok || sc.StateChanged == nil {
		return map[string]string{}
	}
	attrs := sc.StateChanged.GetAttributes()
	if attrs == nil {
		return map[string]string{}
	}
	fields := map[string]string{}
	switch kind := attrs.Kind.(type) {
	case *entityv1.Attributes_Light:
		if kind.Light != nil {
			l := kind.Light
			fields["on"] = fmt.Sprintf("%v", l.On)
			fields["brightness"] = fmt.Sprintf("%d", l.Brightness)
			if l.ColorTemp != 0 {
				fields["color_temp"] = fmt.Sprintf("%d", l.ColorTemp)
			}
			if l.ColorRgb != 0 {
				fields["color_rgb"] = fmt.Sprintf("%d", l.ColorRgb)
			}
		}
	case *entityv1.Attributes_SwitchDevice:
		if kind.SwitchDevice != nil {
			fields["on"] = fmt.Sprintf("%v", kind.SwitchDevice.On)
		}
	case *entityv1.Attributes_NumericSensor:
		if kind.NumericSensor != nil {
			fields["value"] = fmt.Sprintf("%g", kind.NumericSensor.Value)
			fields["unit"] = kind.NumericSensor.Unit
		}
	case *entityv1.Attributes_BinarySensor:
		if kind.BinarySensor != nil {
			fields["on"] = fmt.Sprintf("%v", kind.BinarySensor.On)
		}
	}
	if attrs.Available {
		fields["available"] = "true"
	}
	return fields
}
