package trigger_test

import (
	"testing"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/automation/trigger"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

func driverEv(kind, detail string) eventstore.Event {
	return eventstore.Event{Kind: kind, Payload: &eventv1.Payload{Kind: &eventv1.Payload_DriverEvent{
		DriverEvent: &eventv1.DriverEvent{Kind: kind, Detail: detail},
	}}}
}

func TestEventMatcher_KindOK(t *testing.T) {
	m := trigger.NewEventMatcher("a1", "driver_event", nil)
	mm, ok := m.OnEvent(driverEv("driver_event", ""))
	if !ok || mm.AutomationID != "a1" {
		t.Fatalf("bad: %v %+v", ok, mm)
	}
}

func TestEventMatcher_KindMismatch(t *testing.T) {
	m := trigger.NewEventMatcher("a1", "driver_event", nil)
	if _, ok := m.OnEvent(driverEv("other", "")); ok {
		t.Fatal("should not match")
	}
}

func TestEventMatcher_DataFilter(t *testing.T) {
	m := trigger.NewEventMatcher("a1", "driver_event", map[string]string{"detail": "startup"})
	if _, ok := m.OnEvent(driverEv("driver_event", "startup")); !ok {
		t.Fatal("want match on detail")
	}
	if _, ok := m.OnEvent(driverEv("driver_event", "other")); ok {
		t.Fatal("detail mismatch should not match")
	}
}

func driverEvFull(kind, detail, instanceID, evKind string) eventstore.Event {
	return eventstore.Event{Kind: kind, Payload: &eventv1.Payload{Kind: &eventv1.Payload_DriverEvent{
		DriverEvent: &eventv1.DriverEvent{Kind: evKind, Detail: detail, DriverInstanceId: instanceID},
	}}}
}

func TestEventMatcher_DriverInstanceIDFilter(t *testing.T) {
	m := trigger.NewEventMatcher("a1", "driver_event", map[string]string{"driver_instance_id": "inst1"})
	if _, ok := m.OnEvent(driverEvFull("driver_event", "", "inst1", "")); !ok {
		t.Fatal("want match on driver_instance_id")
	}
	if _, ok := m.OnEvent(driverEvFull("driver_event", "", "inst2", "")); ok {
		t.Fatal("driver_instance_id mismatch should not match")
	}
}

func TestEventMatcher_KindDataFilter(t *testing.T) {
	m := trigger.NewEventMatcher("a1", "driver_event", map[string]string{"kind": "started"})
	if _, ok := m.OnEvent(driverEvFull("driver_event", "", "", "started")); !ok {
		t.Fatal("want match on kind=started")
	}
	if _, ok := m.OnEvent(driverEvFull("driver_event", "", "", "stopped")); ok {
		t.Fatal("kind mismatch should not match")
	}
}

func TestValidateEventDataKeys_UnknownKey(t *testing.T) {
	if err := trigger.ValidateEventDataKeys(map[string]string{"unsupported_key": "x"}); err == nil {
		t.Fatal("want error for unknown data key")
	}
}

func TestValidateEventDataKeys_AllowedKeys(t *testing.T) {
	for k := range trigger.AllowedEventDataKeys {
		if err := trigger.ValidateEventDataKeys(map[string]string{k: "v"}); err != nil {
			t.Fatalf("allowed key %q rejected: %v", k, err)
		}
	}
}

func webhookEv(slug string) eventstore.Event {
	return eventstore.Event{
		Kind: "webhook_received",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_WebhookReceived{
			WebhookReceived: &eventv1.WebhookReceived{Slug: slug},
		}},
	}
}

func TestWebhookMatcher_Match(t *testing.T) {
	m := &trigger.WebhookMatcher{AutomationIDVal: "a1", Path: "foo"}

	if m.AutomationID() != "a1" {
		t.Fatalf("wrong AutomationID: %q", m.AutomationID())
	}
	if m.Kind() != "webhook" {
		t.Fatalf("wrong Kind: %q", m.Kind())
	}
	if m.EventKind() != "webhook_received" {
		t.Fatalf("wrong EventKind: %q", m.EventKind())
	}

	// correct slug matches
	mm, ok := m.OnEvent(webhookEv("foo"))
	if !ok || mm.AutomationID != "a1" || mm.TriggerKind != "webhook" {
		t.Fatalf("expected match: ok=%v mm=%+v", ok, mm)
	}

	// wrong slug does not match
	if _, ok := m.OnEvent(webhookEv("bar")); ok {
		t.Fatal("slug mismatch should not match")
	}
}
