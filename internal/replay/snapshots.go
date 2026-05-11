// Package replay implements the ReplayService: load entity state at any
// sequence number, walk causation chains, and query event windows.
package replay

import (
	"context"
	"maps"
	"time"
)

// EntityStateMap is a map of entity ID → field name → string value.
type EntityStateMap map[string]map[string]string

// Snapshot is a point-in-time entity state image at a given sequence number.
type Snapshot struct {
	Seq      uint64
	Entities EntityStateMap
}

// EntityEvent is an event that updates entity field state.
type EntityEvent struct {
	Seq      uint64
	EntityID string
	Fields   map[string]string // full field set after the event is applied
	// Metadata (populated by EventReader implementations)
	EventID       string
	Kind          string
	Source        string
	CausationID   string
	CorrelationID string
	Emitter       string
	SpanID        string
	OccurredAt    time.Time
	PayloadJSON   string
	WhyInteresting string
}

// SnapshotStore retrieves the nearest snapshot whose sequence is ≤ seq.
type SnapshotStore interface {
	SnapshotBefore(ctx context.Context, seq uint64) (Snapshot, error)
}

// EventReader retrieves entity events in the given (fromSeq, toSeq] range.
type EventReader interface {
	EventsInRange(ctx context.Context, fromSeq, toSeq uint64) ([]EntityEvent, error)
}

// nearestSnapshot returns the snapshot with the greatest Seq ≤ the target.
// If no snapshot exists, it returns a zero-value Snapshot (Seq=0, empty Entities).
func nearestSnapshot(ctx context.Context, store SnapshotStore, seq uint64) (Snapshot, error) {
	return store.SnapshotBefore(ctx, seq)
}

// replayForward applies events in (snap.Seq, targetSeq] to the snapshot's
// entity map and returns the resulting EntityStateMap.
func replayForward(ctx context.Context, reader EventReader, snap Snapshot, targetSeq uint64) (EntityStateMap, error) {
	// Deep-copy the snapshot so mutations don't alias the original.
	result := make(EntityStateMap, len(snap.Entities))
	for entityID, fields := range snap.Entities {
		cp := make(map[string]string, len(fields))
		maps.Copy(cp, fields)
		result[entityID] = cp
	}

	if targetSeq <= snap.Seq {
		return result, nil
	}

	events, err := reader.EventsInRange(ctx, snap.Seq, targetSeq)
	if err != nil {
		return nil, err
	}

	for _, e := range events {
		if _, ok := result[e.EntityID]; !ok {
			result[e.EntityID] = make(map[string]string, len(e.Fields))
		}
		for k, v := range e.Fields {
			result[e.EntityID][k] = v
		}
	}

	return result, nil
}
