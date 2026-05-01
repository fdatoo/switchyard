package testutil

import (
	"time"

	"github.com/google/uuid"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

type EventOption func(*eventstore.Event)

func WithSource(s string) EventOption { return func(e *eventstore.Event) { e.Source = s } }
func WithCorrelation(id uuid.UUID) EventOption {
	return func(e *eventstore.Event) { e.CorrelationID = id }
}
func WithCause(pos uint64) EventOption      { return func(e *eventstore.Event) { e.CausePosition = pos } }
func WithTimestamp(t time.Time) EventOption { return func(e *eventstore.Event) { e.Timestamp = t } }

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

func SystemStartup(opts ...EventOption) eventstore.Event {
	e := eventstore.Event{
		Kind:      "system",
		Source:    "gohomed",
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
