package eventstore_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/registry"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/testutil"
)

type recordedSpan struct {
	name   string
	attrs  map[string]any
	events []string
	errors []error
}

type spanRecorder struct {
	mu    sync.Mutex
	spans []*recordedSpan
}

func (r *spanRecorder) start(ctx context.Context, name string) (context.Context, observability.Span) {
	r.mu.Lock()
	defer r.mu.Unlock()

	span := &recordingSpan{recordedSpan: &recordedSpan{name: name, attrs: map[string]any{}}}
	r.spans = append(r.spans, span.recordedSpan)
	return ctx, span
}

func (r *spanRecorder) snapshot() []recordedSpan {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]recordedSpan, len(r.spans))
	for i, span := range r.spans {
		out[i] = recordedSpan{
			name:   span.name,
			attrs:  map[string]any{},
			events: slices.Clone(span.events),
			errors: slices.Clone(span.errors),
		}
		for key, value := range span.attrs {
			out[i].attrs[key] = value
		}
	}
	return out
}

type recordingSpan struct {
	mu sync.Mutex
	*recordedSpan
}

func (s *recordingSpan) End() {}

func (s *recordingSpan) SetAttr(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs[key] = value
}

func (s *recordingSpan) AddEvent(name string, _ ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, name)
}

func (s *recordingSpan) RecordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, err)
}

func TestTracing_OpenSpansAtSpecCallSites(t *testing.T) {
	ctx := context.Background()
	rec := &spanRecorder{}
	restore := observability.SetSpanStarterForTest(rec.start)
	t.Cleanup(restore)

	f := newStoreFixture(t)
	cache := state.New()
	reg, err := registry.New(ctx, f.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(reg, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}

	_, err = f.store.Append(ctx, entityRegistered("light.trace"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.AppendBatch(ctx, []eventstore.Event{testutil.StateChanged("light.trace", 100)})
	if err != nil {
		t.Fatal(err)
	}

	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{FromPosition: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	for i := 0; i < 2; i++ {
		select {
		case <-sub.C():
		case <-time.After(2 * time.Second):
			t.Fatal("subscription catchup did not deliver history")
		}
	}

	spans := rec.snapshot()
	assertSpan(t, spans, "eventstore.Append", map[string]any{
		"kind":   "entity_registered",
		"entity": "light.trace",
	})
	assertSpan(t, spans, "eventstore.AppendBatch", map[string]any{"batch_size": 1})
	assertSpan(t, spans, "projector.Apply", map[string]any{"projector": "state_cache"})
	assertSpan(t, spans, "projector.Apply", map[string]any{"projector": "registry"})
	assertSpan(t, spans, "state.Apply", nil)
	assertSpan(t, spans, "registry.Apply", nil)
	assertSpan(t, spans, "eventstore.SubscriptionCatchup", nil)
}

func TestTracing_RecordsErrors(t *testing.T) {
	ctx := context.Background()
	rec := &spanRecorder{}
	restore := observability.SetSpanStarterForTest(rec.start)
	t.Cleanup(restore)

	s := newStore(t)
	if _, err := s.Append(ctx, eventstore.Event{}); err == nil {
		t.Fatal("expected Append error")
	}

	spans := rec.snapshot()
	for _, span := range spans {
		if span.name == "eventstore.Append" && len(span.errors) > 0 {
			return
		}
	}
	t.Fatal("eventstore.Append span did not record the error")
}

func assertSpan(t *testing.T, spans []recordedSpan, name string, attrs map[string]any) {
	t.Helper()
	for _, span := range spans {
		if span.name != name {
			continue
		}
		matches := true
		for key, want := range attrs {
			if got := span.attrs[key]; got != want {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	t.Fatalf("missing span %q with attrs %+v; got %+v", name, attrs, spans)
}

func entityRegistered(entity string) eventstore.Event {
	return eventstore.Event{
		Kind:      "entity_registered",
		Entity:    entity,
		Source:    "test",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				DriverInstanceId: "driver.trace",
				EntityType:       "light",
				FriendlyName:     "Trace Light",
				Capabilities: &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
					Light: &entityv1.Light{},
				}},
			},
		}},
	}
}
