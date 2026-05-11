package replay

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	replayv1 "github.com/fdatoo/switchyard/gen/switchyard/replay/v1"
)

// --- fixtures ---

func makeTestStore() *mockStore {
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return &mockStore{
		snaps: []Snapshot{
			{
				Seq: 100,
				Entities: EntityStateMap{
					"light.kitchen": {"brightness": "18"},
				},
			},
		},
		events: []EntityEvent{
			{
				Seq:      101,
				EntityID: "light.kitchen",
				Fields:   map[string]string{"brightness": "18"},
				EventID:  "evt_101",
				Kind:     "state.updated",
				Source:   "driver.hue",
				OccurredAt: t0,
			},
			{
				Seq:        102,
				EntityID:   "light.kitchen",
				Fields:     map[string]string{"brightness": "64"},
				EventID:    "evt_102",
				Kind:       "state.updated",
				Source:     "driver.hue",
				CausationID: "evt_101",
				OccurredAt: t0.Add(time.Second),
			},
			{
				Seq:        103,
				EntityID:   "light.kitchen",
				Fields:     map[string]string{"brightness": "80"},
				EventID:    "evt_103",
				Kind:       "state.updated",
				Source:     "driver.hue",
				CausationID: "evt_102",
				OccurredAt: t0.Add(2 * time.Second),
			},
		},
		causationMap: map[string]string{
			"evt_102": "evt_101",
			"evt_103": "evt_102",
		},
		seqByEventID: map[string]uint64{
			"evt_101": 101,
			"evt_102": 102,
			"evt_103": 103,
		},
	}
}

// mockStore implements SnapshotStore + EventReader + EventLookup.
type mockStore struct {
	snaps        []Snapshot
	events       []EntityEvent
	causationMap map[string]string // event_id -> causation_id
	seqByEventID map[string]uint64
}

func (m *mockStore) SnapshotBefore(ctx context.Context, seq uint64) (Snapshot, error) {
	var best Snapshot
	for _, s := range m.snaps {
		if s.Seq <= seq && s.Seq >= best.Seq {
			best = s
		}
	}
	return best, nil
}

func (m *mockStore) EventsInRange(ctx context.Context, fromSeq, toSeq uint64) ([]EntityEvent, error) {
	var out []EntityEvent
	for _, e := range m.events {
		if e.Seq > fromSeq && e.Seq <= toSeq {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *mockStore) EventByID(ctx context.Context, eventID string) (EntityEvent, bool, error) {
	for _, e := range m.events {
		if e.EventID == eventID {
			return e, true, nil
		}
	}
	return EntityEvent{}, false, nil
}

func (m *mockStore) EventBySeq(ctx context.Context, seq uint64) (EntityEvent, bool, error) {
	for _, e := range m.events {
		if e.Seq == seq {
			return e, true, nil
		}
	}
	return EntityEvent{}, false, nil
}

// --- tests ---

func TestLoadAtSeq_StateDiff(t *testing.T) {
	store := makeTestStore()
	svc := NewService(store, store, store, store)

	// seq 102: brightness changed from 18 → 64
	resp, err := svc.LoadAtSeq(context.Background(), connect.NewRequest(&replayv1.LoadAtSeqRequest{
		Seq: 102,
	}))
	if err != nil {
		t.Fatalf("LoadAtSeq: %v", err)
	}

	diff := resp.Msg.Diff
	if diff == nil {
		t.Fatal("expected non-nil diff")
	}
	if len(diff.EntityDiffs) == 0 {
		t.Fatal("expected at least one entity diff")
	}

	ed := diff.EntityDiffs[0]
	if ed.EntityId != "light.kitchen" {
		t.Errorf("expected entity_id=light.kitchen, got %q", ed.EntityId)
	}
	if len(ed.FieldDiffs) == 0 {
		t.Fatal("expected at least one field diff")
	}
	fd := ed.FieldDiffs[0]
	if fd.Field != "brightness" {
		t.Errorf("expected field=brightness, got %q", fd.Field)
	}
	if fd.Was != "18" {
		t.Errorf("expected was=18, got %q", fd.Was)
	}
	if fd.Now != "64" {
		t.Errorf("expected now=64, got %q", fd.Now)
	}
}

func TestCausationChain_ReturnsChainRootFirst(t *testing.T) {
	store := makeTestStore()
	svc := NewService(store, store, store, store)

	// evt_103 causation: evt_103 ← evt_102 ← evt_101
	resp, err := svc.CausationChain(context.Background(), connect.NewRequest(&replayv1.CausationChainRequest{
		EventId: "evt_103",
	}))
	if err != nil {
		t.Fatalf("CausationChain: %v", err)
	}

	events := resp.Msg.Events
	if len(events) != 3 {
		t.Fatalf("expected 3 chain events, got %d: %v", len(events), events)
	}
	// Root first
	if events[0].EventId != "evt_101" {
		t.Errorf("expected root evt_101, got %q", events[0].EventId)
	}
	if events[2].EventId != "evt_103" {
		t.Errorf("expected tail evt_103, got %q", events[2].EventId)
	}
}

func TestWindow_ExcludesOutOfRange(t *testing.T) {
	store := makeTestStore()
	svc := NewService(store, store, store, store)

	// Window from seq 101 to 102: should include evt_101 and evt_102 only
	resp, err := svc.Window(context.Background(), connect.NewRequest(&replayv1.WindowRequest{
		FromSeq: 100,
		ToSeq:   102,
	}))
	if err != nil {
		t.Fatalf("Window: %v", err)
	}

	events := resp.Msg.Events
	if len(events) != 2 {
		t.Fatalf("expected 2 window events, got %d", len(events))
	}
	if events[0].EventId != "evt_101" {
		t.Errorf("expected evt_101, got %q", events[0].EventId)
	}
	if events[1].EventId != "evt_102" {
		t.Errorf("expected evt_102, got %q", events[1].EventId)
	}
}
