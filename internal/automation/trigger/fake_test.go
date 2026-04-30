package trigger_test

import (
	"testing"

	"github.com/fdatoo/gohome/internal/automation/trigger"
	"github.com/fdatoo/gohome/internal/eventstore"
)

func TestFakeMatcher_EventEmitter(t *testing.T) {
	fm := &trigger.FakeMatcher{ID: "a1", KindVal: "event", EventKindVal: "ping"}
	if fm.AutomationID() != "a1" || fm.Kind() != "event" || fm.EventKind() != "ping" {
		t.Fatalf("unexpected identity: %s/%s/%s", fm.AutomationID(), fm.Kind(), fm.EventKind())
	}

	// Disarmed: no match.
	if _, ok := fm.OnEvent(eventstore.Event{Kind: "ping"}); ok {
		t.Fatal("expected no match while disarmed")
	}

	fm.Arm()
	m, ok := fm.OnEvent(eventstore.Event{Kind: "ping"})
	if !ok {
		t.Fatal("expected match after Arm")
	}
	if m.AutomationID != "a1" || m.TriggerKind != "event" {
		t.Fatalf("bad match: %+v", m)
	}
	if got := fm.FireCount(); got != 1 {
		t.Fatalf("FireCount = %d, want 1", got)
	}
}

func TestFakeMatcher_StateEmitter(t *testing.T) {
	fm := &trigger.FakeMatcher{ID: "a2", KindVal: "state_changed", EntitiesVal: []string{"light.x"}}
	got := []trigger.Match{}
	emit := func(m trigger.Match) { got = append(got, m) }

	fm.OnStateChanged(eventstore.Event{Entity: "light.x"}, emit)
	if len(got) != 0 {
		t.Fatal("expected no emission while disarmed")
	}

	fm.Arm()
	fm.OnStateChanged(eventstore.Event{Entity: "light.x"}, emit)
	if len(got) != 1 || got[0].AutomationID != "a2" {
		t.Fatalf("bad emission: %+v", got)
	}
}

func TestFakeMatcher_ManualFire(t *testing.T) {
	fm := &trigger.FakeMatcher{ID: "a3"}
	got := []trigger.Match{}
	fm.SetDeliverHold(func(m trigger.Match) { got = append(got, m) })
	fm.ManualFire()
	if len(got) != 1 || got[0].AutomationID != "a3" {
		t.Fatalf("ManualFire: got %+v", got)
	}
	if fm.FireCount() != 1 {
		t.Fatalf("FireCount=%d", fm.FireCount())
	}
}
