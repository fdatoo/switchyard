package trigger

import (
	"sync"

	"github.com/fdatoo/switchyard/internal/eventstore"
)

// FakeMatcher is a programmable Matcher for unit tests. It implements both
// StateEmitter and EventEmitter so it can be registered into a Registry
// under either index, but defaults to event-style dispatch (OnEvent returns
// the pre-programmed Match for any incoming event when Armed is true).
//
// Usage:
//
//	fm := &FakeMatcher{ID: "auto1", KindVal: "event", EventKindVal: "ping"}
//	reg.RegisterEvent(fm)
//	reg.Dispatch(eventstore.Event{Kind: "ping"}) // → []Match{{AutomationID: "auto1", TriggerKind: "event"}}
type FakeMatcher struct {
	ID           string
	KindVal      string
	EventKindVal string // for EventEmitter — kind to match
	EntitiesVal  []string

	mu         sync.Mutex
	armed      bool
	deliver    func(Match)
	fireCount  int
	receivedEv []eventstore.Event
}

// AutomationID implements Matcher.
func (m *FakeMatcher) AutomationID() string { return m.ID }

// Kind implements Matcher.
func (m *FakeMatcher) Kind() string {
	if m.KindVal == "" {
		return "event"
	}
	return m.KindVal
}

// Entities implements StateEmitter.
func (m *FakeMatcher) Entities() []string { return m.EntitiesVal }

// EventKind implements EventEmitter.
func (m *FakeMatcher) EventKind() string {
	if m.EventKindVal == "" {
		return "fake"
	}
	return m.EventKindVal
}

// Arm enables OnEvent / OnStateChanged firing. Disarm() reverses.
func (m *FakeMatcher) Arm()    { m.mu.Lock(); m.armed = true; m.mu.Unlock() }
func (m *FakeMatcher) Disarm() { m.mu.Lock(); m.armed = false; m.mu.Unlock() }

// SetDeliverHold mirrors StateChangeMatcher's hold-fire callback so the engine
// can wire deferred delivery in state-emitter mode.
func (m *FakeMatcher) SetDeliverHold(fn func(Match)) {
	m.mu.Lock()
	m.deliver = fn
	m.mu.Unlock()
}

// OnEvent implements EventEmitter. Returns a Match if armed.
func (m *FakeMatcher) OnEvent(ev eventstore.Event) (Match, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.receivedEv = append(m.receivedEv, ev)
	if !m.armed {
		return Match{}, false
	}
	m.fireCount++
	return Match{AutomationID: m.ID, TriggerKind: m.Kind(), Event: &ev}, true
}

// OnStateChanged implements StateEmitter. Calls emit(Match) once per call when armed.
func (m *FakeMatcher) OnStateChanged(ev eventstore.Event, emit func(Match)) {
	m.mu.Lock()
	armed := m.armed
	m.receivedEv = append(m.receivedEv, ev)
	if armed {
		m.fireCount++
	}
	m.mu.Unlock()
	if !armed {
		return
	}
	emit(Match{AutomationID: m.ID, TriggerKind: m.Kind(), Event: &ev})
}

// FireCount returns the number of times this matcher has been triggered.
func (m *FakeMatcher) FireCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fireCount
}

// ManualFire synthesises a Match and delivers it via the registered hold
// callback. Useful for state-emitter style tests that want to drive the
// engine deterministically without going through the eventstore.
func (m *FakeMatcher) ManualFire() {
	m.mu.Lock()
	deliver := m.deliver
	m.fireCount++
	m.mu.Unlock()
	if deliver != nil {
		deliver(Match{AutomationID: m.ID, TriggerKind: m.Kind()})
	}
}
