// Package eventstore owns the SQLite event log, tailer, projector dispatch,
// subscriptions, and snapshots.
package eventstore

import (
	"time"

	"github.com/google/uuid"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
)

// Event is the in-memory Go representation of a row in the events table.
// Position is 0 on input to Append and is populated by the store on return.
// CorrelationID is optional (zero value if none); CausePosition is 0 if none.
type Event struct {
	Position      uint64
	Timestamp     time.Time
	Kind          string
	Entity        string
	Source        string
	CorrelationID uuid.UUID
	CausePosition uint64
	Payload       *eventv1.Payload
}
