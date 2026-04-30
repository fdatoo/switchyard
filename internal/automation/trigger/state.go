package trigger

import (
	"sync"
	"time"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
)

// StateChangeMatcher fires on state-changed events matching (entities, from, to).
// When forDur>0, firing is delayed by a timer; intervening state changes
// that break the predicate cancel the pending timer. Per-entity timers.
type StateChangeMatcher struct {
	automationID string
	entities     map[string]struct{}
	from, to     string
	forDur       time.Duration

	mu          sync.Mutex
	deliverHold func(Match)
	timers      map[string]*time.Timer
	last        map[string]string
}

func NewStateChangeMatcher(id string, entities []string, from, to string, forDur time.Duration, deliverHold func(Match)) *StateChangeMatcher {
	set := make(map[string]struct{}, len(entities))
	for _, e := range entities {
		set[e] = struct{}{}
	}
	return &StateChangeMatcher{
		automationID: id, entities: set, from: from, to: to, forDur: forDur,
		deliverHold: deliverHold,
		timers:      map[string]*time.Timer{},
		last:        map[string]string{},
	}
}

func (m *StateChangeMatcher) AutomationID() string { return m.automationID }
func (m *StateChangeMatcher) Kind() string         { return "state_changed" }
func (m *StateChangeMatcher) Entities() []string {
	out := make([]string, 0, len(m.entities))
	for e := range m.entities {
		out = append(out, e)
	}
	return out
}

// SetDeliverHold wires the hold-fire callback post-construction.
func (m *StateChangeMatcher) SetDeliverHold(fn func(Match)) {
	m.mu.Lock()
	m.deliverHold = fn
	m.mu.Unlock()
}

func (m *StateChangeMatcher) OnStateChanged(ev eventstore.Event, emit func(Match)) {
	if _, ok := m.entities[ev.Entity]; !ok {
		return
	}
	newState := extractStateString(ev)

	m.mu.Lock()
	prev := m.last[ev.Entity]
	m.last[ev.Entity] = newState
	if t, ok := m.timers[ev.Entity]; ok {
		t.Stop()
		delete(m.timers, ev.Entity)
	}
	deliver := m.deliverHold
	m.mu.Unlock()

	if m.from != "" && prev != m.from {
		return
	}
	if m.to != "" && newState != m.to {
		return
	}

	if m.forDur == 0 {
		emit(Match{AutomationID: m.automationID, TriggerKind: "state_changed", Event: &ev})
		return
	}
	if deliver == nil {
		return
	}

	evCopy := ev
	t := time.AfterFunc(m.forDur, func() {
		m.mu.Lock()
		delete(m.timers, evCopy.Entity)
		m.mu.Unlock()
		deliver(Match{AutomationID: m.automationID, TriggerKind: "state_changed", Event: &evCopy})
	})
	m.mu.Lock()
	m.timers[ev.Entity] = t
	m.mu.Unlock()
}

// Stop cancels all pending hold timers.
func (m *StateChangeMatcher) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.timers {
		t.Stop()
	}
	m.timers = map[string]*time.Timer{}
}

func extractStateString(ev eventstore.Event) string {
	p := ev.Payload.GetStateChanged()
	if p == nil || p.GetAttributes() == nil {
		return ""
	}
	a := p.GetAttributes()
	switch k := a.GetKind().(type) {
	case *entityv1.Attributes_Light:
		if k.Light.GetOn() {
			return "on"
		}
		return "off"
	case *entityv1.Attributes_SwitchDevice:
		if k.SwitchDevice.GetOn() {
			return "on"
		}
		return "off"
	}
	return ""
}
