package eventstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func TestReplay_RebuildsStateFromEvents(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache1 := state.New()
	if err := f.store.RegisterProjector(cache1, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 100))
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.b", 50))
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 200))
	t.Cleanup(func() { _ = f.store.Close(ctx) })

	// Second store, same DB: replay must reconstruct state.
	logger := observability.Init(observability.LogConfig{})
	metrics := observability.NewMetrics()
	s2, err := eventstore.Open(ctx, eventstore.Config{}, f.db, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	cache2 := state.New()
	if err := s2.RegisterProjector(cache2, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := s2.Replay(ctx); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	s, ok := cache2.Get("light.a")
	if !ok {
		t.Fatal("light.a missing after replay")
	}
	if s.Attributes.GetLight().Brightness != 200 {
		t.Fatalf("brightness = %d, want 200", s.Attributes.GetLight().Brightness)
	}
}

func TestReplay_ReturnsReplayError(t *testing.T) {
	ctx := context.Background()

	// Populate the DB with one event using a store that has no failing projector.
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	pos, err := f.store.Append(ctx, testutil.StateChanged("light.x", 1))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.store.Close(ctx)

	// Replay on a fresh store with a projector that fails on the first Apply call.
	f2 := newStoreFixtureOnDB(t, f.db)
	if err := f2.store.RegisterProjector(&countingProjector{name: "boom", failAt: 1}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	err = f2.store.Replay(ctx)
	if err == nil {
		t.Fatal("expected replay to fail")
	}
	var re *eventstore.ReplayError
	if !errors.As(err, &re) {
		t.Fatalf("expected *eventstore.ReplayError, got %T: %v", err, err)
	}
	if re.Position != pos {
		t.Fatalf("ReplayError.Position = %d, want %d", re.Position, pos)
	}
	if re.Projector != "boom" {
		t.Fatalf("ReplayError.Projector = %q, want %q", re.Projector, "boom")
	}
	if re.Err == nil {
		t.Fatal("ReplayError.Err must not be nil")
	}
	// Verify the inner error is accessible via the unwrap chain.
	if re.Err == nil || !strings.Contains(re.Err.Error(), "intentional failure") {
		t.Fatalf("expected inner error to contain 'intentional failure', got: %v", re.Err)
	}
}

func TestReplay_SkipEventAllowsReplayToProceed(t *testing.T) {
	ctx := context.Background()

	// Phase A: populate the DB with one event using a store with no failing projector.
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	pos, err := f.store.Append(ctx, testutil.StateChanged("light.z", 1))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("appended event at position %d", pos)
	_ = f.store.Close(ctx)

	// Phase B: replay with a projector that fails on the first Apply → ReplayError.
	f2 := newStoreFixtureOnDB(t, f.db)
	if err := f2.store.RegisterProjector(&countingProjector{name: "boom", failAt: 1}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	replayErr := f2.store.Replay(ctx)
	if replayErr == nil {
		t.Fatal("expected replay to fail")
	}
	var re *eventstore.ReplayError
	if !errors.As(replayErr, &re) {
		t.Fatalf("expected *eventstore.ReplayError, got %T", replayErr)
	}

	// Phase C: insert a skip row for (position, "boom") — simulates POST /events/{pos}/skip.
	_, err = f.db.ExecContext(ctx, `
		INSERT INTO skipped_events (position, projector, skipped_at, skipped_by, reason)
		VALUES (?, ?, ?, ?, ?)`,
		re.Position, "boom", time.Now().UnixNano(), "integration-test", "skip to unblock replay",
	)
	if err != nil {
		t.Fatalf("insert skipped_events: %v", err)
	}

	// Phase D: replay again — must succeed now that the event is skipped.
	f3 := newStoreFixtureOnDB(t, f.db)
	if err := f3.store.RegisterProjector(&countingProjector{name: "boom", failAt: 1}, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f3.store.Replay(ctx); err != nil {
		t.Fatalf("replay after skip failed: %v", err)
	}
}
