package eventstore

import (
	"context"

	"github.com/fdatoo/switchyard/internal/storage"
)

type ProjectorMode int

const (
	ProjectorModeSync ProjectorMode = iota
	ProjectorModeAsync
)

// Projector materializes a view of the event log. Sync projectors run
// inside Append's transaction; async projectors run off the tailer.
type Projector interface {
	Name() string
	Apply(ctx context.Context, tx storage.Tx, e Event) error
	Snapshot(ctx context.Context, tx storage.Tx) error
	Restore(ctx context.Context, tx storage.Tx) (resumeFrom uint64, err error)
}

// PostCommit is implemented by projectors that need to promote in-memory
// state after the Append transaction commits. Called by the store in
// registration order; errors are logged and ignored (log is source of truth).
type PostCommit interface {
	Promote()
}

// Discarder is implemented by projectors that need to drop tx-local state
// on rollback.
type Discarder interface {
	Discard()
}

// NoSnapshot is embeddable for projectors whose state lives entirely
// in SQL (e.g., registry). Restore returns 0, meaning "read cursor
// from projection_cursors".
type NoSnapshot struct{}

func (NoSnapshot) Snapshot(context.Context, storage.Tx) error          { return nil }
func (NoSnapshot) Restore(context.Context, storage.Tx) (uint64, error) { return 0, nil }
