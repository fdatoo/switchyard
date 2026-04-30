// Package trigger defines the runtime trigger-matching surface. The
// automation compiler builds Matchers; the Registry indexes them for O(1)
// dispatch from the event-store subscription loop.
package trigger

import (
	"sync"

	"github.com/fdatoo/gohome/internal/eventstore"
)

// Match is one fired trigger hit threaded through the mode state machine.
type Match struct {
	AutomationID string
	TriggerKind  string // "state_changed" | "event" | "time" | "manual"
	Event        *eventstore.Event
}

// Matcher is the base interface for every kind of matcher.
type Matcher interface {
	AutomationID() string
	Kind() string
}

// StateEmitter is the richer interface for state-change matchers.
type StateEmitter interface {
	Matcher
	Entities() []string
	OnStateChanged(ev eventstore.Event, emit func(Match))
}

// EventEmitter is the richer interface for non-state event matchers.
type EventEmitter interface {
	Matcher
	EventKind() string
	OnEvent(ev eventstore.Event) (Match, bool)
}

// Registry indexes matchers by entity (for state) or kind (for events).
type Registry struct {
	mu            sync.RWMutex
	stateByEntity map[string][]StateEmitter
	eventByKind   map[string][]EventEmitter
}

func NewRegistry() *Registry {
	return &Registry{
		stateByEntity: map[string][]StateEmitter{},
		eventByKind:   map[string][]EventEmitter{},
	}
}

func (r *Registry) RegisterState(m StateEmitter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range m.Entities() {
		r.stateByEntity[e] = append(r.stateByEntity[e], m)
	}
}

func (r *Registry) RegisterEvent(m EventEmitter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventByKind[m.EventKind()] = append(r.eventByKind[m.EventKind()], m)
}

func (r *Registry) Unregister(m Matcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch s := m.(type) {
	case StateEmitter:
		for _, e := range s.Entities() {
			r.stateByEntity[e] = dropState(r.stateByEntity[e], s)
		}
	case EventEmitter:
		k := s.EventKind()
		r.eventByKind[k] = dropEvent(r.eventByKind[k], s)
	}
}

// Dispatch fans an inbound event out to every applicable matcher. State
// matchers receive an emit callback so forDur==0 fires land in the returned
// slice and forDur>0 scheduling stays inside the matcher.
func (r *Registry) Dispatch(ev eventstore.Event) []Match {
	var out []Match
	r.mu.RLock()
	switch ev.Kind {
	case "state_changed":
		for _, m := range r.stateByEntity[ev.Entity] {
			m.OnStateChanged(ev, func(mm Match) { out = append(out, mm) })
		}
	default:
		for _, m := range r.eventByKind[ev.Kind] {
			if mm, ok := m.OnEvent(ev); ok {
				out = append(out, mm)
			}
		}
	}
	r.mu.RUnlock()
	return out
}

func dropState(list []StateEmitter, target StateEmitter) []StateEmitter {
	out := list[:0]
	for _, m := range list {
		if m != target {
			out = append(out, m)
		}
	}
	return out
}

func dropEvent(list []EventEmitter, target EventEmitter) []EventEmitter {
	out := list[:0]
	for _, m := range list {
		if m != target {
			out = append(out, m)
		}
	}
	return out
}
