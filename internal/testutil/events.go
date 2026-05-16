package testutil

import (
	"time"

	"github.com/google/uuid"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

// EventOption mutates a synthesized test event.
type EventOption func(*eventstore.Event)

// WithSource overrides the event source.
func WithSource(s string) EventOption { return func(e *eventstore.Event) { e.Source = s } }

// WithCorrelation overrides the event correlation id.
func WithCorrelation(id uuid.UUID) EventOption {
	return func(e *eventstore.Event) { e.CorrelationID = id }
}

// WithCause sets the event cause position.
func WithCause(pos uint64) EventOption { return func(e *eventstore.Event) { e.CausePosition = pos } }

// WithTimestamp overrides the event timestamp.
func WithTimestamp(t time.Time) EventOption { return func(e *eventstore.Event) { e.Timestamp = t } }

// StateChanged builds a light state_changed event for tests.
func StateChanged(entity string, brightness uint32, opts ...EventOption) eventstore.Event {
	e := eventstore.Event{
		Kind:      "state_changed",
		Entity:    entity,
		Source:    "test",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_StateChanged{
			StateChanged: &eventv1.StateChanged{
				Attributes: &entityv1.Attributes{
					Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{
						On: brightness > 0, Brightness: brightness,
					}},
				},
			},
		}},
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}

// SystemStartup builds a system startup event for tests.
func SystemStartup(opts ...EventOption) eventstore.Event {
	e := eventstore.Event{
		Kind:      "system",
		Source:    "switchyardd",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_System{
			System: &eventv1.SystemEvent{Kind: "startup"},
		}},
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}
