// Package eventstoretest provides a fake in-memory Appender for unit tests.
package eventstoretest

import (
	"context"
	"sync"
	"testing"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

// EventStore is a fake Appender that collects appended events in memory.
type EventStore struct {
	mu     sync.Mutex
	events []eventstore.Event
}

// New returns a new in-memory EventStore. t is accepted to signal test-only
// intent and to mark the call site in failure output via t.Helper.
func New(t *testing.T) *EventStore {
	t.Helper()
	return &EventStore{}
}

// AppendAuth implements eventstore.Appender.
func (es *EventStore) AppendAuth(ctx context.Context, e *eventv1.AuthEvent) error {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.events = append(es.events, eventstore.Event{
		Kind:    "auth_event",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AuthEvent{AuthEvent: e}},
	})
	return nil
}

// All returns a copy of all collected events.
func (es *EventStore) All() []eventstore.Event {
	es.mu.Lock()
	defer es.mu.Unlock()
	out := make([]eventstore.Event, len(es.events))
	copy(out, es.events)
	return out
}
