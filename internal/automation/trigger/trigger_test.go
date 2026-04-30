package trigger_test

import (
	"testing"

	"github.com/fdatoo/gohome/internal/automation/trigger"
	"github.com/fdatoo/gohome/internal/eventstore"
)

func TestRegistry_DispatchEmpty(t *testing.T) {
	r := trigger.NewRegistry()
	got := r.Dispatch(eventstore.Event{Kind: "state_changed", Entity: "light.x"})
	if len(got) != 0 {
		t.Fatalf("got %d", len(got))
	}
}
