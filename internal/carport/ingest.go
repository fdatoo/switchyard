package carport

import (
	"context"
	"fmt"
	"time"

	carportpb "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	eventpb "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
)

// IngestMessage translates a non-Result DriverToHost message to an eventstore
// Event and appends it. Returns the append error verbatim; caller is
// responsible for tearing down the Run stream on error (enforces INV-2).
//
// The entity ID is derived from the message payload itself: StateChanged
// carries it explicitly, EntityRegistered/EntityUnregistered use DeviceId,
// DriverEvent has no entity binding.
func IngestMessage(ctx context.Context, store *eventstore.Store, instanceID string, msg *carportpb.DriverToHost) error {
	now := time.Now()
	source := "driver:" + instanceID

	switch k := msg.GetKind().(type) {
	case *carportpb.DriverToHost_StateChanged:
		_, err := store.Append(ctx, eventstore.Event{
			Timestamp: now,
			Kind:      "state_changed",
			Entity:    k.StateChanged.GetEntityId(),
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_StateChanged{
				StateChanged: k.StateChanged,
			}},
		})
		return err

	case *carportpb.DriverToHost_EntityRegistered:
		er := k.EntityRegistered
		if er != nil && er.DriverInstanceId == "" {
			er.DriverInstanceId = instanceID
		}
		_, err := store.Append(ctx, eventstore.Event{
			Timestamp: now,
			Kind:      "entity_registered",
			Entity:    er.GetDeviceId(),
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityRegistered{
				EntityRegistered: er,
			}},
		})
		return err

	case *carportpb.DriverToHost_EntityUnregistered:
		_, err := store.Append(ctx, eventstore.Event{
			Timestamp: now,
			Kind:      "entity_unregistered",
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityUnregistered{
				EntityUnregistered: k.EntityUnregistered,
			}},
		})
		return err

	case *carportpb.DriverToHost_DriverEvent:
		de := k.DriverEvent
		if de != nil && de.DriverInstanceId == "" {
			de.DriverInstanceId = instanceID
		}
		_, err := store.Append(ctx, eventstore.Event{
			Timestamp: now,
			Kind:      "driver_event",
			Source:    source,
			Payload: &eventpb.Payload{Kind: &eventpb.Payload_DriverEvent{
				DriverEvent: de,
			}},
		})
		return err

	case *carportpb.DriverToHost_Pong:
		return nil

	case *carportpb.DriverToHost_Result:
		return fmt.Errorf("IngestMessage: Result should be consumed by instanceConn.deliver, not ingested")

	default:
		return fmt.Errorf("IngestMessage: unknown message kind %T", msg.GetKind())
	}
}
