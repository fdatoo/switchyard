package eventstore

import (
	"time"

	"github.com/google/uuid"
)

// Filter selects events. All populated fields AND together;
// within a slice field, any match succeeds. Zero time bounds are
// treated as unbounded.
type Filter struct {
	Kinds          []string
	Entities       []string
	Sources        []string
	CorrelationIDs []uuid.UUID
	MinTs, MaxTs   time.Time
}

func (f Filter) Matches(e Event) bool {
	if len(f.Kinds) > 0 && !containsString(f.Kinds, e.Kind) {
		return false
	}
	if len(f.Entities) > 0 && !containsString(f.Entities, e.Entity) {
		return false
	}
	if len(f.Sources) > 0 && !containsString(f.Sources, e.Source) {
		return false
	}
	if len(f.CorrelationIDs) > 0 && !containsUUID(f.CorrelationIDs, e.CorrelationID) {
		return false
	}
	if !f.MinTs.IsZero() && e.Timestamp.Before(f.MinTs) {
		return false
	}
	if !f.MaxTs.IsZero() && e.Timestamp.After(f.MaxTs) {
		return false
	}
	return true
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func containsUUID(haystack []uuid.UUID, needle uuid.UUID) bool {
	for _, u := range haystack {
		if u == needle {
			return true
		}
	}
	return false
}
