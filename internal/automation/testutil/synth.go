package testutil

import (
	"context"
	"sync"
	"time"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
)

type FakeState struct {
	mu sync.RWMutex
	m  map[string]*ghstarlark.EntityState
}

func NewFakeState() *FakeState { return &FakeState{m: map[string]*ghstarlark.EntityState{}} }

func (f *FakeState) Get(id string) (*ghstarlark.EntityState, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	v, ok := f.m[id]
	return v, ok
}

func (f *FakeState) Set(id string, s *ghstarlark.EntityState) {
	f.mu.Lock()
	f.m[id] = s
	f.mu.Unlock()
}

type FakeDispatcher struct {
	mu     sync.Mutex
	Calls  []DispatchCall
	Result *ghstarlark.DispatchResult
	Err    error
}

type DispatchCall struct {
	Entity, Capability string
	Args               map[string]string
}

func (f *FakeDispatcher) Dispatch(_ context.Context, entity, cap string, args map[string]string) (*ghstarlark.DispatchResult, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, DispatchCall{entity, cap, args})
	f.mu.Unlock()
	if f.Result != nil || f.Err != nil {
		return f.Result, f.Err
	}
	return &ghstarlark.DispatchResult{Ok: true}, nil
}

// GetCalls returns a safe copy of all recorded dispatch calls.
func (f *FakeDispatcher) GetCalls() []DispatchCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]DispatchCall, len(f.Calls))
	copy(out, f.Calls)
	return out
}

// FakeEventStore satisfies automation.EventStore for unit tests that don't
// want a real sqlite store.
type FakeEventStore struct {
	mu     sync.Mutex
	Events []eventstore.Event
	subs   []*fakeSub
	pos    uint64
}

func (f *FakeEventStore) Append(_ context.Context, e eventstore.Event) (uint64, error) {
	f.mu.Lock()
	f.pos++
	e.Position = f.pos
	f.Events = append(f.Events, e)
	subs := append([]*fakeSub(nil), f.subs...)
	f.mu.Unlock()
	for _, s := range subs {
		s.trySend(e)
	}
	return e.Position, nil
}

func (f *FakeEventStore) LatestPosition() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pos
}

// CopyEvents returns a safe copy of the event slice for test assertions.
func (f *FakeEventStore) CopyEvents() []eventstore.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]eventstore.Event, len(f.Events))
	copy(out, f.Events)
	return out
}

func (f *FakeEventStore) Subscribe(ctx context.Context, opts eventstore.SubscribeOptions) (eventstore.Subscription, error) {
	s := &fakeSub{
		ch:     make(chan eventstore.Event, 64),
		closed: make(chan struct{}),
		filter: opts.Filter,
	}
	f.mu.Lock()
	// Replay matching events with Position > FromPosition into the subscriber
	// buffer before registering for live broadcasts. Best-effort: if the
	// buffer fills, replays drop silently (real store would error with
	// ErrSubscriptionOverflow; the fake is forgiving for unit tests).
	for _, e := range f.Events {
		if e.Position <= opts.FromPosition {
			continue
		}
		if !opts.Filter.Matches(e) {
			continue
		}
		select {
		case s.ch <- e:
		default:
		}
	}
	f.subs = append(f.subs, s)
	f.mu.Unlock()
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()
	return s, nil
}

type fakeSub struct {
	ch     chan eventstore.Event
	closed chan struct{}
	once   sync.Once
	filter eventstore.Filter

	// sendMu guards isClosed and serialises send-vs-close so that Append
	// never sends to a closed channel. Append holds sendMu during the
	// non-blocking send; Close holds sendMu while flipping isClosed and
	// closing ch (after closing the sentinel closed channel).
	sendMu   sync.Mutex
	isClosed bool
}

func (s *fakeSub) C() <-chan eventstore.Event { return s.ch }
func (s *fakeSub) Ack(uint64) error           { return nil }

// Close marks the subscription as closed and closes both sentinel channels.
// Closing s.closed first ensures Append's select guard fires on next call.
// sendMu is held during the ch-close so a concurrent Append cannot send to a
// closed channel.
func (s *fakeSub) Close() error {
	s.once.Do(func() {
		s.sendMu.Lock()
		s.isClosed = true
		close(s.closed)
		close(s.ch)
		s.sendMu.Unlock()
	})
	return nil
}

// trySend delivers e to the subscriber if it is still open and matches the
// subscriber's filter. Returns false only if the subscriber was already closed
// (filtered-out events are dropped silently and the subscriber stays alive).
func (s *fakeSub) trySend(e eventstore.Event) bool {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if s.isClosed {
		return false
	}
	if !s.filter.Matches(e) {
		return true
	}
	select {
	case s.ch <- e:
	default:
	}
	return true
}

func (s *fakeSub) Stats() eventstore.SubscriptionStats { return eventstore.SubscriptionStats{} }

// MakeLightStateEvent creates a synthetic state_changed event for a light.
func MakeLightStateEvent(entity string, on bool) eventstore.Event {
	return eventstore.Event{
		Kind:      "state_changed",
		Entity:    entity,
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_StateChanged{
			StateChanged: &eventv1.StateChanged{Attributes: &entityv1.Attributes{
				Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{On: on}},
			}},
		}},
	}
}
