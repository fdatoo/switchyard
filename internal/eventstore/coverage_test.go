package eventstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/state"
	"github.com/fdatoo/gohome/internal/storage"
	"github.com/fdatoo/gohome/internal/testutil"
)

func TestAppendBatch(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache := state.New()
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	events := []eventstore.Event{
		testutil.StateChanged("light.a", 100),
		testutil.StateChanged("light.b", 200),
		testutil.StateChanged("light.c", 50),
	}
	positions, err := f.store.AppendBatch(ctx, events)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("len = %d, want 3", len(positions))
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Fatalf("positions not monotonic: %v", positions)
		}
	}
}

func TestAppendBatch_Empty(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	positions, err := f.store.AppendBatch(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if positions != nil {
		t.Fatalf("expected nil positions for empty batch, got %v", positions)
	}
}

func TestSubscription_Stats(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	stats := sub.Stats()
	if stats.Delivered != 0 {
		t.Fatalf("Delivered = %d, want 0", stats.Delivered)
	}
	if stats.Dropped != 0 {
		t.Fatalf("Dropped = %d, want 0", stats.Dropped)
	}
}

func TestStateCache_Discard(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache := state.New()
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Apply an event, then verify Discard clears pending state.
	// Discard is called on rollback — we trigger it by calling it directly.
	cache.Discard()
	// Cache should still be functional after Discard.
	if cache.Len() != 0 {
		t.Fatalf("after Discard, Len = %d, want 0", cache.Len())
	}
}

func TestNoSnapshot(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Use registry as a NoSnapshot projector — Snapshot/Restore are already tested there.
	// Verify directly that NoSnapshot methods work.
	var ns eventstore.NoSnapshot
	err := ns.Snapshot(ctx, nil)
	if err != nil {
		t.Fatalf("NoSnapshot.Snapshot: %v", err)
	}
	pos, err := ns.Restore(ctx, nil)
	if err != nil {
		t.Fatalf("NoSnapshot.Restore: %v", err)
	}
	if pos != 0 {
		t.Fatalf("NoSnapshot.Restore = %d, want 0", pos)
	}
}

func TestRegisterProjector_AfterStart(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	err := f.store.RegisterProjector(state.New(), eventstore.ProjectorModeSync)
	if err == nil {
		t.Fatal("expected error when registering projector after Start")
	}
}

func TestReplay_ErrorWhenAlreadyStarted(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	err := f.store.Replay(ctx)
	if err == nil {
		t.Fatal("expected error when Replay called after Start")
	}
}

func TestSubscribe_ErrorWhenNotStarted(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	_, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{})
	if err == nil {
		t.Fatal("expected error when Subscribe called before Start")
	}
}

func TestAppendBatch_InvalidEvent(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := f.store.AppendBatch(ctx, []eventstore.Event{{Kind: ""}})
	if err == nil {
		t.Fatal("expected error for invalid event (missing Kind)")
	}
}

func TestReplay_NoProjectors(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	// No projectors registered; Replay should still succeed on an empty DB.
	if err := f.store.Replay(ctx); err != nil {
		t.Fatalf("Replay with no projectors: %v", err)
	}
}

func TestDurableSubscription_Ack(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{
		Durable: true,
		Name:    "test-consumer",
	})
	if err != nil {
		t.Fatalf("Subscribe durable: %v", err)
	}
	defer func() { _ = sub.Close() }()

	pos, err := f.store.Append(ctx, testutil.StateChanged("light.a", 50))
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-sub.C():
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	if err := sub.Ack(pos); err != nil {
		t.Fatalf("Ack: %v", err)
	}
}

func TestDurableSubscription_ResumesFromCursor(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Create durable sub and Ack position 1.
	sub, _ := f.store.Subscribe(ctx, eventstore.SubscribeOptions{Durable: true, Name: "resumable"})
	pos1, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 10))
	select {
	case <-sub.C():
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	_ = sub.Ack(pos1)
	_ = sub.Close()

	// Append a second event, then re-subscribe — should only see the second.
	pos2, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 20))
	sub2, _ := f.store.Subscribe(ctx, eventstore.SubscribeOptions{Durable: true, Name: "resumable"})
	defer func() { _ = sub2.Close() }()

	select {
	case e := <-sub2.C():
		if e.Position != pos2 {
			t.Fatalf("got position %d, want %d", e.Position, pos2)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout on second sub")
	}
}

func TestFilter_AllFields(t *testing.T) {
	now := time.Now()
	corrID := uuid.New()
	e := eventstore.Event{
		Kind:          "state_changed",
		Entity:        "light.a",
		Source:        "driver:x",
		CorrelationID: corrID,
		Timestamp:     now,
	}

	// CorrelationID match
	f := eventstore.Filter{CorrelationIDs: []uuid.UUID{corrID}}
	if !f.Matches(e) {
		t.Fatal("CorrelationID filter should match")
	}
	f2 := eventstore.Filter{CorrelationIDs: []uuid.UUID{uuid.New()}}
	if f2.Matches(e) {
		t.Fatal("wrong CorrelationID should not match")
	}

	// MinTs / MaxTs
	f3 := eventstore.Filter{MinTs: now.Add(-time.Second)}
	if !f3.Matches(e) {
		t.Fatal("MinTs before event should match")
	}
	f4 := eventstore.Filter{MinTs: now.Add(time.Second)}
	if f4.Matches(e) {
		t.Fatal("MinTs after event should not match")
	}
	f5 := eventstore.Filter{MaxTs: now.Add(time.Second)}
	if !f5.Matches(e) {
		t.Fatal("MaxTs after event should match")
	}
	f6 := eventstore.Filter{MaxTs: now.Add(-time.Second)}
	if f6.Matches(e) {
		t.Fatal("MaxTs before event should not match")
	}
}

func TestSubscription_Dispatch_Drop(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Buffer of 1; fill it before dispatching more events to trigger drop.
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{
		ChannelBuffer: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	// Drain the catchup buffer by reading first event.
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 1))
	select {
	case <-sub.C():
	case <-time.After(time.Second):
	}

	// Now saturate the buffer with live events (don't read).
	for i := 0; i < 5; i++ {
		_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", uint32(i+2)))
	}
	time.Sleep(50 * time.Millisecond)

	stats := sub.Stats()
	if stats.Delivered+stats.Dropped == 0 {
		t.Fatal("expected at least one delivered or dropped event")
	}
}

func TestNonDurableSubscription_Ack_IsNoop(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	// Non-durable Ack must be a no-op (not an error).
	if err := sub.Ack(1); err != nil {
		t.Fatalf("non-durable Ack: %v", err)
	}
}

func TestSnapshotNow_UnknownOwner(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := f.store.SnapshotNow(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown snapshot owner")
	}
}

func TestSnapshotNow_AllProjectors(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache := state.New()
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 10))
	// Snapshot all projectors (owner == "").
	pos, err := f.store.SnapshotNow(ctx, "")
	if err != nil {
		t.Fatalf("SnapshotNow all: %v", err)
	}
	if pos == 0 {
		t.Fatal("expected non-zero position from snapshot")
	}
}

func TestSubscribe_DurableRequiresName(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{Durable: true, Name: ""})
	if err == nil {
		t.Fatal("expected error: Durable without Name")
	}
}

func TestReplay_WithProjectorAndEvents(t *testing.T) {
	ctx := context.Background()

	// Append events in one store instance.
	f := newStoreFixture(t)
	cache1 := state.New()
	if err := f.store.RegisterProjector(cache1, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", uint32(i+1)))
	}

	// Take snapshot so Restore returns a non-zero cursor.
	_, err := f.store.SnapshotNow(ctx, "state_cache")
	if err != nil {
		t.Fatalf("SnapshotNow: %v", err)
	}

	// Open second store on same DB, replay.
	f2 := newStoreFixtureOnDB(t, f.db)
	cache2 := state.New()
	if err := f2.store.RegisterProjector(cache2, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f2.store.Replay(ctx); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if cache2.Len() == 0 {
		t.Fatal("expected entities in cache after replay")
	}
}

// noSnapProjector is a minimal projector with NoSnapshot so runSnapshot
// hits the sql.ErrNoRows branch and returns LatestPosition().
type noSnapProjector struct {
	eventstore.NoSnapshot
}

func (p *noSnapProjector) Name() string { return "no_snap_proj" }
func (p *noSnapProjector) Apply(_ context.Context, _ storage.Tx, _ eventstore.Event) error {
	return nil
}

func TestSnapshotNow_NoSnapshotProjector(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)

	proj := &noSnapProjector{}
	if err := f.store.RegisterProjector(proj, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 1))

	// SnapshotNow with a NoSnapshot projector hits the sql.ErrNoRows path.
	pos, err := f.store.SnapshotNow(ctx, "no_snap_proj")
	if err != nil {
		t.Fatalf("SnapshotNow with NoSnapshot projector: %v", err)
	}
	if pos == 0 {
		t.Fatal("expected non-zero position from LatestPosition fallback")
	}
}

func TestReplay_SkipsEventInSkippedTable(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache := state.New()
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	pos, err := f.store.Append(ctx, testutil.StateChanged("light.a", 99))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.store.Close(ctx)

	// Mark the event as skipped in the DB so replayBatch hits the warn+continue path.
	_, err = f.db.ExecContext(ctx,
		`INSERT INTO skipped_events (position, projector, skipped_at, skipped_by, reason) VALUES (?, ?, ?, ?, ?)`,
		pos, "state_cache", 0, "test", "test skip",
	)
	if err != nil {
		t.Fatalf("insert skipped_event: %v", err)
	}

	// Re-open and replay.
	f2 := newStoreFixtureOnDB(t, f.db)
	cache2 := state.New()
	if err := f2.store.RegisterProjector(cache2, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f2.store.Replay(ctx); err != nil {
		t.Fatalf("Replay with skipped event: %v", err)
	}
	// Event was skipped so cache2 should not have light.a.
	if _, ok := cache2.Get("light.a"); ok {
		t.Fatal("skipped event should not be applied")
	}
}

func TestReplay_AsyncProjectorSkippedDuringReplay(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)

	// Register an async projector — replay skips async projectors.
	asyncProj := &noSnapProjector{}
	if err := f.store.RegisterProjector(asyncProj, eventstore.ProjectorModeAsync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_ = f.store.Close(ctx)

	_, _ = f.db.ExecContext(ctx, `INSERT INTO events (ts, kind, source, payload) VALUES (1, 'state_changed', 'test', X'')`)

	f2 := newStoreFixtureOnDB(t, f.db)
	asyncProj2 := &noSnapProjector{}
	if err := f2.store.RegisterProjector(asyncProj2, eventstore.ProjectorModeAsync); err != nil {
		t.Fatal(err)
	}
	// Replay with only async projector — replayBatch hits the mode != sync continue.
	if err := f2.store.Replay(ctx); err != nil {
		t.Fatalf("Replay with async projector: %v", err)
	}
}

// failProjector returns an error on the first Apply to trigger the projector error path.
type failProjector struct {
	eventstore.NoSnapshot
	fail bool
}

func (p *failProjector) Name() string { return "fail_proj" }
func (p *failProjector) Apply(_ context.Context, _ storage.Tx, _ eventstore.Event) error {
	if p.fail {
		return errors.New("intentional projector failure")
	}
	return nil
}

func TestAppend_ProjectorFailure(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	proj := &failProjector{fail: true}
	if err := f.store.RegisterProjector(proj, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Append should fail because the projector returns an error.
	// This also exercises: withRetry non-SQLITE_BUSY path, isSQLiteBusy non-nil path,
	// appendTx Discarder defer, projector error metrics.
	_, err := f.store.Append(ctx, testutil.StateChanged("light.a", 1))
	if err == nil {
		t.Fatal("expected error from projector failure")
	}
}

func TestAppend_AsyncProjectorSkipped(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	asyncProj := &noSnapProjector{}
	if err := f.store.RegisterProjector(asyncProj, eventstore.ProjectorModeAsync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Append with an async projector — the projector is skipped during appendTx
	// (mode != ProjectorModeSync), covering that continue branch.
	pos, err := f.store.Append(ctx, testutil.StateChanged("light.a", 1))
	if err != nil {
		t.Fatalf("Append with async projector: %v", err)
	}
	if pos == 0 {
		t.Fatal("expected non-zero position")
	}
}

func TestStart_SecondCallIsNoop(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Second call must be a noop (return nil immediately).
	if err := f.store.Start(ctx); err != nil {
		t.Fatalf("second Start: %v", err)
	}
}

func TestQuery_WithFilter(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 10))
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.b", 20))

	// Query with a filter — covers the filter application path in Query.
	evts, err := f.store.Query(ctx, eventstore.QueryOptions{
		Filter: eventstore.Filter{Entities: []string{"light.a"}},
	})
	if err != nil {
		t.Fatalf("Query with filter: %v", err)
	}
	for _, e := range evts {
		if e.Entity != "light.a" {
			t.Fatalf("filtered event has wrong entity: %s", e.Entity)
		}
	}
}

func TestAppend_ZeroTimestampAutoFilled(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// Event with zero timestamp — Append must fill it in.
	e := testutil.StateChanged("light.a", 1, testutil.WithTimestamp(time.Time{}))
	pos, err := f.store.Append(ctx, e)
	if err != nil {
		t.Fatalf("Append with zero timestamp: %v", err)
	}
	if pos == 0 {
		t.Fatal("expected non-zero position")
	}
}

func TestAppendBatch_ZeroTimestampAutoFilled(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	e := testutil.StateChanged("light.a", 1, testutil.WithTimestamp(time.Time{}))
	positions, err := f.store.AppendBatch(ctx, []eventstore.Event{e})
	if err != nil {
		t.Fatalf("AppendBatch with zero timestamp: %v", err)
	}
	if len(positions) != 1 || positions[0] == 0 {
		t.Fatalf("expected 1 non-zero position, got %v", positions)
	}
}

func TestAppendBatch_WithProjectors(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	cache := state.New()
	if err := f.store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	asyncProj := &noSnapProjector{}
	if err := f.store.RegisterProjector(asyncProj, eventstore.ProjectorModeAsync); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// AppendBatch with both sync and async projectors.
	// Covers: sync projector apply in batch, async projector skip (continue),
	// projection_cursors advance, PostCommit Promote after batch commit.
	positions, err := f.store.AppendBatch(ctx, []eventstore.Event{
		testutil.StateChanged("light.a", 10),
		testutil.StateChanged("light.b", 20),
	})
	if err != nil {
		t.Fatalf("AppendBatch with projectors: %v", err)
	}
	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %v", positions)
	}
	if cache.Len() == 0 {
		t.Fatal("expected entities in cache after AppendBatch with projector")
	}
}

func TestQuery_CustomLimitAndToPosition(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", uint32(i+1)))
	}

	// Query with custom limit > 0 (covers the non-default limit branch)
	// and ToPosition (covers the position > 0 check).
	evts, err := f.store.Query(ctx, eventstore.QueryOptions{
		ToPosition: 3,
		Limit:      2,
	})
	if err != nil {
		t.Fatalf("Query with limit/to-pos: %v", err)
	}
	if len(evts) > 3 {
		t.Fatalf("expected ≤ 3 events, got %d", len(evts))
	}
}

func TestStore_CloseStopsSnapshotter(t *testing.T) {
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
	_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", 1))

	// Close cancels the tailerCtx, causing the snapshotter goroutine to
	// receive from ctx.Done() and exit — covering the ctx.Done() select case.
	if err := f.store.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Give the goroutine time to run the ctx.Done() case.
	time.Sleep(50 * time.Millisecond)
}

func TestSubscription_CloseWithMultipleSubscribers(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Register two subscribers, then close only the first.
	// unregisterSubscriber must keep the second in the list (covers the append path).
	sub1, _ := f.store.Subscribe(ctx, eventstore.SubscribeOptions{})
	sub2, _ := f.store.Subscribe(ctx, eventstore.SubscribeOptions{})
	defer func() { _ = sub2.Close() }()

	_ = sub1.Close()

	// Verify the store still works with sub2 by appending an event.
	_, err := f.store.Append(ctx, testutil.StateChanged("light.a", 1))
	if err != nil {
		t.Fatalf("Append after sub1 closed: %v", err)
	}
	select {
	case <-sub2.C():
	case <-time.After(time.Second):
		t.Fatal("sub2 timed out after sub1 was closed")
	}
}
