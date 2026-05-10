package carport_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	carportpb "github.com/fdatoo/switchyard/gen/switchyard/carport/v1alpha1"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
)

func TestDispatch_EntityUnknown(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, nil)
	defer h.Stop(context.Background())

	_, err := h.Dispatch(context.Background(), "light.nope", "turn_on", nil)
	if !errors.Is(err, carport.ErrEntityUnknown) {
		t.Fatalf("got %v, want ErrEntityUnknown", err)
	}
	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if countByKind(evs, "command_issued")+countByKind(evs, "command_ack") > 0 {
		t.Fatal("no command events should be appended on pre-flight error")
	}
}

func TestDispatch_InstanceNotRunning(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	defer h.Stop(context.Background())

	_, err := h.Dispatch(context.Background(), "light.kitchen", "turn_on", nil)
	if !errors.Is(err, carport.ErrInstanceNotRunning) {
		t.Fatalf("got %v, want ErrInstanceNotRunning", err)
	}
	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if countByKind(evs, "command_issued")+countByKind(evs, "command_ack") > 0 {
		t.Fatal("no command events should be appended on pre-flight error")
	}
}

func TestDispatch_HappyPathAppendsIssuedAndAck(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := h.Dispatch(ctx, "light.kitchen", "turn_on", map[string]string{"brightness": "60"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !res.Ok {
		t.Error("expected ok=true")
	}

	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if issued, acked := countByKind(evs, "command_issued"), countByKind(evs, "command_ack"); issued != 1 || acked != 1 {
		t.Errorf("issued=%d acked=%d, want 1/1", issued, acked)
	}
}

func TestDispatch_TracingSpanLifecycle(t *testing.T) {
	rec := &dispatchSpanRecorder{}
	restore := observability.SetSpanStarterForTest(rec.start)
	t.Cleanup(restore)

	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := h.Dispatch(ctx, "light.kitchen", "turn_on", map[string]string{"brightness": "60"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	span, ok := rec.find("carport.Dispatch")
	if !ok {
		t.Fatal("missing carport.Dispatch span")
	}
	if !span.ended {
		t.Fatal("carport.Dispatch span was not ended")
	}

	wantAttrs := map[string]any{
		"instance_id": "hue_main",
		"entity_id":   "light.kitchen",
		"capability":  "turn_on",
		"command_id":  res.CommandId,
	}
	for key, want := range wantAttrs {
		if got := span.attrs[key]; got != want {
			t.Fatalf("span attr %q = %v, want %v", key, got, want)
		}
	}

	wantEvents := []string{
		"CommandIssued appended",
		"sent on stream",
		"CommandResult received",
		"CommandAck appended",
	}
	if !slices.Equal(span.events, wantEvents) {
		t.Fatalf("span events = %v, want %v", span.events, wantEvents)
	}
}

func TestDispatch_TimeoutAppendsAck(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		time.Sleep(200 * time.Millisecond)
		return &carportpb.CommandResult{CommandId: c.CommandId, Ok: true}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := h.Dispatch(ctx, "light.kitchen", "turn_on", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, carport.ErrDispatchTimeout) {
		t.Errorf("expected timeout error, got %v", err)
	}

	evs, _ := f.store.Query(context.Background(), anyQueryOptions())
	if issued, acked := countByKind(evs, "command_issued"), countByKind(evs, "command_ack"); issued != 1 || acked != 1 {
		t.Errorf("issued=%d acked=%d, want 1/1 (INV-1)", issued, acked)
	}
}

func TestDispatch_DriverReportsError(t *testing.T) {
	f := newStoreFixtureForTest(t)
	h := newHostForDispatch(t, f, seedHueMainEntity)
	stopFake := injectRunningFake(t, h, "hue_main", func(c *carportpb.Command) *carportpb.CommandResult {
		return &carportpb.CommandResult{
			CommandId:    c.CommandId,
			Ok:           false,
			Code:         carportpb.CarportErrorCode_CARPORT_DEVICE_OFFLINE,
			ErrorMessage: "bulb offline",
		}
	})
	defer stopFake()
	defer h.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := h.Dispatch(ctx, "light.kitchen", "turn_on", nil)
	if err != nil {
		t.Fatalf("Dispatch err = %v, want nil (driver-typed failure should return result, nil)", err)
	}
	if res.Ok {
		t.Error("expected ok=false")
	}
	if res.ErrorMessage != "bulb offline" {
		t.Errorf("ErrorMessage = %q", res.ErrorMessage)
	}
}

func countByKind(evs []eventstore.Event, kind string) int {
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

type dispatchRecordedSpan struct {
	name   string
	attrs  map[string]any
	events []string
	ended  bool
}

type dispatchSpanRecorder struct {
	mu    sync.Mutex
	spans []*dispatchRecordedSpan
}

func (r *dispatchSpanRecorder) start(ctx context.Context, name string) (context.Context, observability.Span) {
	r.mu.Lock()
	defer r.mu.Unlock()

	span := &dispatchRecordingSpan{
		recorded: &dispatchRecordedSpan{
			name:  name,
			attrs: map[string]any{},
		},
	}
	r.spans = append(r.spans, span.recorded)
	return ctx, span
}

func (r *dispatchSpanRecorder) find(name string) (dispatchRecordedSpan, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, span := range r.spans {
		if span.name != name {
			continue
		}
		out := dispatchRecordedSpan{
			name:   span.name,
			attrs:  map[string]any{},
			events: slices.Clone(span.events),
			ended:  span.ended,
		}
		for key, value := range span.attrs {
			out.attrs[key] = value
		}
		return out, true
	}
	return dispatchRecordedSpan{}, false
}

type dispatchRecordingSpan struct {
	mu       sync.Mutex
	recorded *dispatchRecordedSpan
}

func (s *dispatchRecordingSpan) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded.ended = true
}

func (s *dispatchRecordingSpan) SetAttr(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded.attrs[key] = value
}

func (s *dispatchRecordingSpan) AddEvent(name string, _ ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded.events = append(s.recorded.events, name)
}

func (s *dispatchRecordingSpan) RecordError(error) {}
