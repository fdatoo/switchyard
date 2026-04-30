package trigger_test

import (
	"testing"
	"time"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/automation/trigger"
	"github.com/fdatoo/gohome/internal/eventstore"
)

func stateEv(entity, s string) eventstore.Event {
	attrs := &entityv1.Attributes{}
	if s == "on" {
		attrs.Kind = &entityv1.Attributes_Light{Light: &entityv1.Light{On: true}}
	}
	if s == "off" {
		attrs.Kind = &entityv1.Attributes_Light{Light: &entityv1.Light{On: false}}
	}
	return eventstore.Event{
		Kind: "state_changed", Entity: entity,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_StateChanged{
			StateChanged: &eventv1.StateChanged{Attributes: attrs},
		}},
	}
}

func TestStateMatcher_ToOn(t *testing.T) {
	var got []trigger.Match
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 0, nil)
	m.OnStateChanged(stateEv("light.a", "on"), func(mm trigger.Match) { got = append(got, mm) })
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
}

func TestStateMatcher_WrongEntity(t *testing.T) {
	var got []trigger.Match
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 0, nil)
	m.OnStateChanged(stateEv("light.b", "on"), func(mm trigger.Match) { got = append(got, mm) })
	if len(got) != 0 {
		t.Fatal("should not match")
	}
}

func TestStateMatcher_ForDurFires(t *testing.T) {
	ch := make(chan trigger.Match, 1)
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 40*time.Millisecond,
		func(mm trigger.Match) { ch <- mm })
	m.OnStateChanged(stateEv("light.a", "on"), func(trigger.Match) { t.Fatal("should not fire synchronously") })
	select {
	case <-ch:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no hold fire")
	}
}

func TestStateMatcher_ForDurCancels(t *testing.T) {
	ch := make(chan trigger.Match, 1)
	m := trigger.NewStateChangeMatcher("a1", []string{"light.a"}, "", "on", 40*time.Millisecond,
		func(mm trigger.Match) { ch <- mm })
	m.OnStateChanged(stateEv("light.a", "on"), func(trigger.Match) {})
	m.OnStateChanged(stateEv("light.a", "off"), func(trigger.Match) {})
	select {
	case <-ch:
		t.Fatal("should not fire after break")
	case <-time.After(80 * time.Millisecond):
	}
}
