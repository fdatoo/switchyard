package trigger

import (
	"fmt"

	"github.com/fdatoo/switchyard/internal/eventstore"
)

// EventMatcher matches non-state events by exact kind and optional data
// equality. Supported data keys for DriverEvent are "detail",
// "driver_instance_id", and "kind". Unknown keys are rejected at compile time
// by ValidateEventDataKeys.
type EventMatcher struct {
	automationID string
	kind         string
	data         map[string]string
}

// AllowedEventDataKeys is the set of DriverEvent fields that can be filtered.
var AllowedEventDataKeys = map[string]struct{}{
	"detail":             {},
	"driver_instance_id": {},
	"kind":               {},
}

// ValidateEventDataKeys returns an error if data contains any key not in
// AllowedEventDataKeys. Call this at compile time to reject unsupported filters
// before they silently ignore declared data keys at runtime.
func ValidateEventDataKeys(data map[string]string) error {
	for k := range data {
		if _, ok := AllowedEventDataKeys[k]; !ok {
			return fmt.Errorf("event trigger: unsupported data key %q (allowed: detail, driver_instance_id, kind)", k)
		}
	}
	return nil
}

func NewEventMatcher(id, kind string, data map[string]string) *EventMatcher {
	return &EventMatcher{automationID: id, kind: kind, data: data}
}

func (m *EventMatcher) AutomationID() string { return m.automationID }
func (m *EventMatcher) Kind() string         { return "event" }
func (m *EventMatcher) EventKind() string    { return m.kind }

func (m *EventMatcher) OnEvent(ev eventstore.Event) (Match, bool) {
	if ev.Kind != m.kind {
		return Match{}, false
	}
	if len(m.data) > 0 {
		de := ev.Payload.GetDriverEvent()
		if de == nil {
			return Match{}, false
		}
		if want, ok := m.data["detail"]; ok && de.GetDetail() != want {
			return Match{}, false
		}
		if want, ok := m.data["driver_instance_id"]; ok && de.GetDriverInstanceId() != want {
			return Match{}, false
		}
		if want, ok := m.data["kind"]; ok && de.GetKind() != want {
			return Match{}, false
		}
	}
	return Match{AutomationID: m.automationID, TriggerKind: "event", Event: &ev}, true
}

// WebhookMatcher fires when a WebhookReceived event arrives with the matching slug.
type WebhookMatcher struct {
	AutomationIDVal string
	Path            string // the slug registered in Pkl config
}

func (w *WebhookMatcher) AutomationID() string { return w.AutomationIDVal }
func (w *WebhookMatcher) Kind() string         { return "webhook" }
func (w *WebhookMatcher) EventKind() string    { return "webhook_received" }

func (w *WebhookMatcher) OnEvent(ev eventstore.Event) (Match, bool) {
	wr := ev.Payload.GetWebhookReceived()
	if wr == nil || wr.GetSlug() != w.Path {
		return Match{}, false
	}
	return Match{AutomationID: w.AutomationIDVal, TriggerKind: "webhook", Event: &ev}, true
}
