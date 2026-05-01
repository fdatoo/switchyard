package eventstore_test

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func TestSnapshotNow_WritesRowForOwner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newStoreFixture(t)
	cache := state.New()
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.store.Close(ctx) })

	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 100))

	pos, err := f.store.SnapshotNow(ctx, "state_cache")
	if err != nil {
		t.Fatalf("SnapshotNow: %v", err)
	}
	if pos == 0 {
		t.Fatal("expected non-zero position")
	}

	var count int
	if err := f.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM snapshots WHERE owner = 'state_cache'`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("snapshot row count = %d, want 1", count)
	}
}
