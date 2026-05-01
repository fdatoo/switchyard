package eventstore_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fdatoo/switchyard/internal/eventstore"
)

func TestFilter_MatchesEmptyFilterMatchesEverything(t *testing.T) {
	f := eventstore.Filter{}
	e := eventstore.Event{Kind: "state_changed", Entity: "light.lr"}
	if !f.Matches(e) {
		t.Fatal("empty filter should match")
	}
}

func TestFilter_MatchesKind(t *testing.T) {
	f := eventstore.Filter{Kinds: []string{"state_changed"}}
	if !f.Matches(eventstore.Event{Kind: "state_changed"}) {
		t.Fatal("kind should match")
	}
	if f.Matches(eventstore.Event{Kind: "command_issued"}) {
		t.Fatal("different kind should not match")
	}
}

func TestFilter_MatchesEntity(t *testing.T) {
	f := eventstore.Filter{Entities: []string{"light.lr"}}
	if f.Matches(eventstore.Event{Entity: "light.kitchen"}) {
		t.Fatal("different entity should not match")
	}
	if !f.Matches(eventstore.Event{Entity: "light.lr"}) {
		t.Fatal("entity should match")
	}
}

func TestFilter_MatchesCorrelationID(t *testing.T) {
	id := uuid.New()
	f := eventstore.Filter{CorrelationIDs: []uuid.UUID{id}}
	if !f.Matches(eventstore.Event{CorrelationID: id}) {
		t.Fatal("correlation id should match")
	}
	if f.Matches(eventstore.Event{CorrelationID: uuid.New()}) {
		t.Fatal("different correlation should not match")
	}
}

func TestFilter_MatchesTimeRange(t *testing.T) {
	t0 := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	f := eventstore.Filter{MinTs: t0}
	if f.Matches(eventstore.Event{Timestamp: t0.Add(-time.Second)}) {
		t.Fatal("before MinTs should not match")
	}
	if !f.Matches(eventstore.Event{Timestamp: t0.Add(time.Second)}) {
		t.Fatal("after MinTs should match")
	}
}

func TestFilter_MatchesAllOfMulti(t *testing.T) {
	f := eventstore.Filter{
		Kinds:    []string{"state_changed"},
		Entities: []string{"light.lr"},
	}
	if !f.Matches(eventstore.Event{Kind: "state_changed", Entity: "light.lr"}) {
		t.Fatal("should match both")
	}
	if f.Matches(eventstore.Event{Kind: "state_changed", Entity: "switch.k"}) {
		t.Fatal("mismatched entity should not match")
	}
}
