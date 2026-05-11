package replay

import (
	"context"
	"testing"
	"time"
)

// --- in-memory SnapshotStore for tests ---

type memSnapshotStore struct {
	snaps []Snapshot
}

func (m *memSnapshotStore) SnapshotBefore(ctx context.Context, seq uint64) (Snapshot, error) {
	var best Snapshot
	for _, s := range m.snaps {
		if s.Seq <= seq && s.Seq >= best.Seq {
			best = s
		}
	}
	return best, nil
}

// --- in-memory EventReader for tests ---

type memEventReader struct {
	events []EntityEvent
}

func (m *memEventReader) EventsInRange(ctx context.Context, fromSeq, toSeq uint64) ([]EntityEvent, error) {
	var out []EntityEvent
	for _, e := range m.events {
		if e.Seq > fromSeq && e.Seq <= toSeq {
			out = append(out, e)
		}
	}
	return out, nil
}

// --- tests ---

func TestNearestSnapshot_NoSnapshots(t *testing.T) {
	store := &memSnapshotStore{}
	snap, err := nearestSnapshot(context.Background(), store, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Seq != 0 {
		t.Errorf("expected zero snapshot, got seq=%d", snap.Seq)
	}
	if len(snap.Entities) != 0 {
		t.Errorf("expected empty entities, got %v", snap.Entities)
	}
}

func TestNearestSnapshot_ReturnsBestLeq(t *testing.T) {
	store := &memSnapshotStore{
		snaps: []Snapshot{
			{Seq: 100, Entities: EntityStateMap{"light.kitchen": {"brightness": "18"}}},
			{Seq: 200, Entities: EntityStateMap{"light.kitchen": {"brightness": "99"}}},
		},
	}
	snap, err := nearestSnapshot(context.Background(), store, 150)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Seq != 100 {
		t.Errorf("expected snap seq=100, got %d", snap.Seq)
	}
}

func TestReplayForward_AppliesEvents(t *testing.T) {
	snap := Snapshot{
		Seq:      100,
		Entities: EntityStateMap{"light.kitchen": {"brightness": "18"}},
	}
	events := []EntityEvent{
		{Seq: 101, EntityID: "light.kitchen", Fields: map[string]string{"brightness": "18"}},
		{Seq: 102, EntityID: "light.kitchen", Fields: map[string]string{"brightness": "30"}},
		{Seq: 103, EntityID: "light.kitchen", Fields: map[string]string{"brightness": "64"}},
		{Seq: 104, EntityID: "light.kitchen", Fields: map[string]string{"on": "true"}},
		{Seq: 105, EntityID: "light.kitchen", Fields: map[string]string{"brightness": "80"}},
	}
	reader := &memEventReader{events: events}

	// Replay to seq 103: brightness should be 64
	result103, err := replayForward(context.Background(), reader, snap, 103)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := result103["light.kitchen"]["brightness"]; got != "64" {
		t.Errorf("seq 103: expected brightness=64, got %q", got)
	}

	// Replay to seq 101: brightness should stay 18
	result101, err := replayForward(context.Background(), reader, snap, 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := result101["light.kitchen"]["brightness"]; got != "18" {
		t.Errorf("seq 101: expected brightness=18, got %q", got)
	}
}

func TestReplayForward_SnapAtTarget(t *testing.T) {
	snap := Snapshot{
		Seq:      100,
		Entities: EntityStateMap{"dev": {"val": "original"}},
	}
	reader := &memEventReader{} // no events
	result, err := replayForward(context.Background(), reader, snap, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := result["dev"]["val"]; got != "original" {
		t.Errorf("expected original, got %q", got)
	}
}

// ensures EntityEvent.Timestamp is used without compile error
var _ = time.Time{}
