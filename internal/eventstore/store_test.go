package eventstore_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/storage"
	"github.com/fdatoo/gohome/internal/testutil"
)

type storeFixture struct {
	store *eventstore.Store
	db    *sql.DB
}

func newStoreFixture(t *testing.T) *storeFixture {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return &storeFixture{store: s, db: db}
}

func newStore(t *testing.T) *eventstore.Store {
	t.Helper()
	return newStoreFixture(t).store
}

func newStoreFixtureOnDB(t *testing.T, db *sql.DB) *storeFixture {
	t.Helper()
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return &storeFixture{store: s, db: db}
}

func TestAppend_AssignsMonotonicPositions(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	p1, err := s.Append(ctx, testutil.StateChanged("light.a", 100))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.Append(ctx, testutil.StateChanged("light.b", 50))
	if err != nil {
		t.Fatal(err)
	}
	if p2 <= p1 {
		t.Fatalf("positions not monotonic: p1=%d p2=%d", p1, p2)
	}
}

func TestAppend_RejectsInvalid(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if _, err := s.Append(ctx, eventstore.Event{Kind: ""}); err == nil {
		t.Fatal("expected error for empty kind")
	}
}

func TestAppendBatch_AtomicAllOrNothing(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	events := []eventstore.Event{
		testutil.StateChanged("light.a", 100),
		testutil.StateChanged("light.b", 200),
		testutil.StateChanged("light.c", 0),
	}
	positions, err := s.AppendBatch(ctx, events)
	if err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("positions len = %d, want 3", len(positions))
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] != positions[i-1]+1 {
			t.Fatalf("non-contiguous batch positions: %v", positions)
		}
	}
}

func TestQuery_ReturnsInOrderAndFilters(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 10))
	_, _ = s.Append(ctx, testutil.StateChanged("light.b", 20))
	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 30))

	got, err := s.Query(ctx, eventstore.QueryOptions{Filter: eventstore.Filter{Entities: []string{"light.a"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("filter returned %d events, want 2", len(got))
	}
	if got[0].Position >= got[1].Position {
		t.Fatal("results not ordered by position")
	}
}

type countingProjector struct {
	name   string
	count  int
	lastE  eventstore.Event
	failAt int
}

func (c *countingProjector) Name() string { return c.name }
func (c *countingProjector) Apply(_ context.Context, _ storage.Tx, e eventstore.Event) error {
	c.count++
	c.lastE = e
	if c.failAt > 0 && c.count == c.failAt {
		return errors.New("intentional failure")
	}
	return nil
}
func (c *countingProjector) Snapshot(context.Context, storage.Tx) error          { return nil }
func (c *countingProjector) Restore(context.Context, storage.Tx) (uint64, error) { return 0, nil }

func TestAppend_SyncProjectorSeesEventInsideTx(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	proj := &countingProjector{name: "test"}
	if err := s.RegisterProjector(proj, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}

	_, err := s.Append(ctx, testutil.StateChanged("light.a", 10))
	if err != nil {
		t.Fatal(err)
	}
	if proj.count != 1 {
		t.Fatalf("projector call count = %d, want 1", proj.count)
	}
	if proj.lastE.Position == 0 {
		t.Fatal("projector received Event with zero Position")
	}
}

func TestAppend_SyncProjectorFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	proj := &countingProjector{name: "fails", failAt: 1}
	if err := s.RegisterProjector(proj, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(ctx, testutil.StateChanged("light.a", 10)); err == nil {
		t.Fatal("expected projector failure to bubble up")
	}
	got, err := s.Query(ctx, eventstore.QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("rolled-back event still present: %+v", got)
	}
}
