package state_test

import (
	"context"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func TestCache_SnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)

	c1 := state.New()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = c1.Apply(ctx, tx, mkStateChanged("light.a", true, 100))
	_ = c1.Apply(ctx, tx, mkStateChanged("light.b", false, 0))
	c1.Promote()

	// Insert a snapshot row.
	if err := c1.Snapshot(ctx, tx); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// Fresh cache restores from the row.
	c2 := state.New()
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := c2.Restore(ctx, tx2)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatal(err)
	}

	if pos == 0 {
		t.Fatal("Restore returned position 0")
	}
	if c2.Len() != 2 {
		t.Fatalf("restored Len = %d, want 2", c2.Len())
	}
	s, ok := c2.Get("light.a")
	if !ok {
		t.Fatal("light.a missing after restore")
	}
	if s.Attributes.GetLight().Brightness != 100 {
		t.Fatalf("brightness = %d, want 100", s.Attributes.GetLight().Brightness)
	}
}

func TestCache_RestoreUnknownEncoding(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)

	// Directly insert a snapshot row with an unsupported encoding.
	_, err := db.ExecContext(ctx, `
		INSERT INTO snapshots (position, ts, owner, encoding, state) VALUES (1, ?, 'state_cache', 'msgpack', X'1234')`,
		time.Now().UnixNano(),
	)
	if err != nil {
		t.Fatalf("insert bad snapshot: %v", err)
	}

	c := state.New()
	tx, _ := db.BeginTx(ctx, nil)
	_, err = c.Restore(ctx, tx)
	_ = tx.Rollback()
	if err == nil {
		t.Fatal("expected error for unknown encoding")
	}
}

func TestCache_RestoreWithNoSnapshotReturnsZero(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	c := state.New()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := c.Restore(ctx, tx)
	if err != nil {
		t.Fatalf("Restore on empty: %v", err)
	}
	if pos != 0 {
		t.Fatalf("expected 0, got %d", pos)
	}
	_ = tx.Rollback()
}
