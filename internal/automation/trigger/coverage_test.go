package trigger_test

import (
	"context"
	"testing"
	"time"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/automation/trigger"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

// --- Registry: covers RegisterState/Event, Unregister branches, Dispatch default + state branches ---

func TestRegistry_RegisterAndDispatchEvent(t *testing.T) {
	r := trigger.NewRegistry()
	em := trigger.NewEventMatcher("a1", "ping", nil)
	r.RegisterEvent(em)
	got := r.Dispatch(eventstore.Event{Kind: "ping"})
	if len(got) != 1 || got[0].AutomationID != "a1" {
		t.Fatalf("dispatch = %+v", got)
	}
}

func TestRegistry_RegisterAndDispatchState(t *testing.T) {
	r := trigger.NewRegistry()
	sm := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 0, nil)
	r.RegisterState(sm)
	got := r.Dispatch(stateEv("light.a", "on"))
	if len(got) != 1 || got[0].AutomationID != "a1" {
		t.Fatalf("dispatch = %+v", got)
	}
}

func TestRegistry_UnregisterEvent(t *testing.T) {
	r := trigger.NewRegistry()
	keep := trigger.NewEventMatcher("keep", "ping", nil)
	drop := trigger.NewEventMatcher("drop", "ping", nil)
	r.RegisterEvent(keep)
	r.RegisterEvent(drop)
	r.Unregister(drop)
	got := r.Dispatch(eventstore.Event{Kind: "ping"})
	if len(got) != 1 || got[0].AutomationID != "keep" {
		t.Fatalf("after unregister: %+v", got)
	}
}

func TestRegistry_UnregisterState(t *testing.T) {
	r := trigger.NewRegistry()
	keep := trigger.NewStateChangeMatcher("keep", []string{"light.a"}, "", "on", 0, nil)
	drop := trigger.NewStateChangeMatcher("drop", []string{"light.a"}, "", "on", 0, nil)
	r.RegisterState(keep)
	r.RegisterState(drop)
	r.Unregister(drop)
	got := r.Dispatch(stateEv("light.a", "on"))
	if len(got) != 1 || got[0].AutomationID != "keep" {
		t.Fatalf("after unregister: %+v", got)
	}
}

func TestRegistry_DispatchEventNoMatch(t *testing.T) {
	r := trigger.NewRegistry()
	r.RegisterEvent(trigger.NewEventMatcher("a1", "ping", nil))
	if got := r.Dispatch(eventstore.Event{Kind: "other_kind"}); len(got) != 0 {
		t.Fatalf("expected no match, got %+v", got)
	}
}

// --- StateChangeMatcher: AutomationID/Kind/Entities/SetDeliverHold/Stop ---

func TestStateMatcher_Accessors(t *testing.T) {
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a", "light.b"}, "off", "on", 0, nil)
	if m.AutomationID() != "a1" {
		t.Errorf("AutomationID=%q", m.AutomationID())
	}
	if m.Kind() != "state_changed" {
		t.Errorf("Kind=%q", m.Kind())
	}
	got := m.Entities()
	if len(got) != 2 {
		t.Fatalf("Entities len=%d", len(got))
	}
}

func TestStateMatcher_StopCancelsPendingTimer(t *testing.T) {
	fired := make(chan struct{}, 1)
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 50*time.Millisecond,
		func(trigger.Match) { fired <- struct{}{} })
	m.OnStateChanged(stateEv("light.a", "on"), func(trigger.Match) {})
	m.Stop()
	select {
	case <-fired:
		t.Fatal("should not have fired after Stop")
	case <-time.After(120 * time.Millisecond):
	}
}

func TestStateMatcher_SetDeliverHoldOverride(t *testing.T) {
	first := make(chan struct{}, 1)
	second := make(chan struct{}, 1)
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 30*time.Millisecond,
		func(trigger.Match) { first <- struct{}{} })
	m.SetDeliverHold(func(trigger.Match) { second <- struct{}{} })
	m.OnStateChanged(stateEv("light.a", "on"), func(trigger.Match) {})
	select {
	case <-second:
	case <-first:
		t.Fatal("hold should have used the overridden callback")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no fire")
	}
}

func TestStateMatcher_FromMustMatchPrior(t *testing.T) {
	var got []trigger.Match
	emit := func(mm trigger.Match) { got = append(got, mm) }
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "off", "on", 0, nil)
	// First call: prev was unset, from="off" doesn't match "" → no fire
	m.OnStateChanged(stateEv("light.a", "on"), emit)
	if len(got) != 0 {
		t.Fatalf("expected no fire on first transition, got %d", len(got))
	}
	// Reset to off, then back to on — now prev=="off", from="off" matches → fire
	m.OnStateChanged(stateEv("light.a", "off"), emit)
	m.OnStateChanged(stateEv("light.a", "on"), emit)
	if len(got) != 1 {
		t.Fatalf("expected one fire after off→on, got %d", len(got))
	}
}

func TestStateMatcher_HoldWithoutDeliverNoFire(t *testing.T) {
	// forDur > 0 but no deliverHold → silently drops
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 30*time.Millisecond, nil)
	m.OnStateChanged(stateEv("light.a", "on"), func(trigger.Match) {})
	time.Sleep(60 * time.Millisecond)
	// no panic, no fire — pass
}

// --- EventMatcher: accessor methods ---

func TestEventMatcher_Accessors(t *testing.T) {
	m := trigger.NewEventMatcher("a2", "alarm", map[string]string{"detail": "tamper"})
	if m.AutomationID() != "a2" {
		t.Errorf("AutomationID=%q", m.AutomationID())
	}
	if m.Kind() != "event" {
		t.Errorf("Kind=%q", m.Kind())
	}
	if m.EventKind() != "alarm" {
		t.Errorf("EventKind=%q", m.EventKind())
	}
}

func TestEventMatcher_DataDetailMismatch(t *testing.T) {
	m := trigger.NewEventMatcher("a1", "driver_event", map[string]string{"detail": "x"})
	ev := eventstore.Event{Kind: "driver_event", Payload: &eventv1.Payload{Kind: &eventv1.Payload_DriverEvent{
		DriverEvent: &eventv1.DriverEvent{Detail: "y"},
	}}}
	if _, ok := m.OnEvent(ev); ok {
		t.Fatal("should not match on detail mismatch")
	}
}

func TestEventMatcher_DataMissingPayload(t *testing.T) {
	m := trigger.NewEventMatcher("a1", "driver_event", map[string]string{"detail": "x"})
	ev := eventstore.Event{Kind: "driver_event"} // no payload
	if _, ok := m.OnEvent(ev); ok {
		t.Fatal("should not match when payload is nil")
	}
}

// --- WebhookMatcher: slug mismatch + nil payload ---

func TestWebhookMatcher_SlugMismatch(t *testing.T) {
	w := &trigger.WebhookMatcher{AutomationIDVal: "a1", Path: "doorbell"}
	ev := eventstore.Event{Kind: "webhook_received", Payload: &eventv1.Payload{Kind: &eventv1.Payload_WebhookReceived{
		WebhookReceived: &eventv1.WebhookReceived{Slug: "garage"},
	}}}
	if _, ok := w.OnEvent(ev); ok {
		t.Fatal("slug mismatch should not match")
	}
}

func TestWebhookMatcher_AccessorsAndNilPayload(t *testing.T) {
	w := &trigger.WebhookMatcher{AutomationIDVal: "a1", Path: "doorbell"}
	if w.AutomationID() != "a1" || w.Kind() != "webhook" || w.EventKind() != "webhook_received" {
		t.Errorf("accessors: id=%q kind=%q event_kind=%q", w.AutomationID(), w.Kind(), w.EventKind())
	}
	if _, ok := w.OnEvent(eventstore.Event{Kind: "webhook_received"}); ok {
		t.Fatal("nil payload should not match")
	}
}

// --- TimeMatcher accessors ---

func TestTimeMatcher_Accessors(t *testing.T) {
	tm := &trigger.TimeMatcher{AutomationIDVal: "a1", At: "08:00"}
	if tm.AutomationID() != "a1" || tm.Kind() != "time" {
		t.Errorf("accessors: id=%q kind=%q", tm.AutomationID(), tm.Kind())
	}
}

// --- TimeScheduler edge paths ---

func TestTimeScheduler_AddEveryRejectsNonPositive(t *testing.T) {
	s := trigger.NewTimeScheduler(time.UTC)
	if err := s.AddEvery("a1", 0); err == nil {
		t.Fatal("AddEvery(0) should error")
	}
	if err := s.AddEvery("a1", -1); err == nil {
		t.Fatal("AddEvery(<0) should error")
	}
}

func TestTimeScheduler_AddCronInvalidExpr(t *testing.T) {
	s := trigger.NewTimeScheduler(time.UTC)
	if err := s.AddCron("a1", "not a cron"); err == nil {
		t.Fatal("AddCron should error on bad expr")
	}
}

func TestTimeScheduler_AddAtBadFormat(t *testing.T) {
	s := trigger.NewTimeScheduler(time.UTC)
	if err := s.AddAt("a1", "garbage"); err == nil {
		t.Fatal("AddAt should error on garbage")
	}
}

func TestTimeScheduler_AddAtOutOfRange(t *testing.T) {
	s := trigger.NewTimeScheduler(time.UTC)
	if err := s.AddAt("a1", "25:00"); err == nil {
		t.Fatal("AddAt should reject hour 25")
	}
	if err := s.AddAt("a1", "10:99"); err == nil {
		t.Fatal("AddAt should reject minute 99")
	}
}

func TestTimeScheduler_NilLocationDefaultsLocal(t *testing.T) {
	s := trigger.NewTimeScheduler(nil)
	if s == nil {
		t.Fatal("nil scheduler")
	}
}

func TestTimeScheduler_ResetClearsEntries(t *testing.T) {
	s := trigger.NewTimeScheduler(time.UTC)
	if err := s.AddEvery("a1", 50*time.Millisecond); err != nil {
		t.Fatalf("AddEvery: %v", err)
	}
	s.Reset()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	go s.Run(ctx)
	select {
	case <-s.Ready():
		t.Fatal("entries should have been cleared by Reset")
	case <-ctx.Done():
	}
}

// --- FakeMatcher: Entities, Disarm, EventKind default ---

func TestFakeMatcher_EntitiesAndDisarm(t *testing.T) {
	fm := &trigger.FakeMatcher{ID: "f1", EntitiesVal: []string{"light.x", "light.y"}}
	if got := fm.Entities(); len(got) != 2 {
		t.Errorf("Entities len=%d", len(got))
	}
	fm.Arm()
	if _, ok := fm.OnEvent(eventstore.Event{Kind: "fake"}); !ok {
		t.Fatal("armed: expected match")
	}
	fm.Disarm()
	if _, ok := fm.OnEvent(eventstore.Event{Kind: "fake"}); ok {
		t.Fatal("disarmed: should not match")
	}
}

func TestFakeMatcher_EventKindDefault(t *testing.T) {
	fm := &trigger.FakeMatcher{ID: "f1"}
	if fm.EventKind() != "fake" {
		t.Errorf("default EventKind=%q, want %q", fm.EventKind(), "fake")
	}
}
