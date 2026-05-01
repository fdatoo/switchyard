package eventstore_test

import (
	"context"
	"fmt"
	"testing"
	"testing/quick"
	"time"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func TestProperty_AppendPositionsMonotonic(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	fn := func(n uint8) bool {
		if n == 0 {
			return true
		}
		prev := f.store.LatestPosition()
		for i := 0; i < int(n); i++ {
			pos, err := f.store.Append(ctx, testutil.StateChanged("light.x", uint32(i)))
			if err != nil {
				return false
			}
			if pos <= prev {
				return false
			}
			prev = pos
		}
		return true
	}
	if err := quick.Check(fn, nil); err != nil {
		t.Fatal(err)
	}
}

func TestProperty_LiveEqualsReplay(t *testing.T) {
	ctx := context.Background()

	// Apply events live.
	f1 := newStoreFixture(t)
	cache1 := state.New()
	if err := f1.store.RegisterProjector(cache1, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := f1.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	events := testutil.LoadFixture(t, "basic_state_flow")
	for _, e := range events {
		if _, err := f1.store.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	liveDump := dumpCache(cache1)

	// Reopen same DB and replay.
	logger := observability.Init(observability.LogConfig{})
	metrics := observability.NewMetrics()
	s2, err := eventstore.Open(ctx, eventstore.Config{}, f1.db, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	cache2 := state.New()
	if err := s2.RegisterProjector(cache2, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := s2.Replay(ctx); err != nil {
		t.Fatal(err)
	}
	replayDump := dumpCache(cache2)

	if !deepEqual(liveDump, replayDump) {
		t.Fatalf("live != replay\nlive: %#v\nreplay: %#v", liveDump, replayDump)
	}
}

func TestProperty_FilterMatchesSubscription(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}
	filter := eventstore.Filter{Entities: []string{"light.a"}}
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	e1 := testutil.StateChanged("light.a", 10)
	e2 := testutil.StateChanged("light.b", 20)
	_, _ = f.store.Append(ctx, e1)
	_, _ = f.store.Append(ctx, e2)

	got := make(map[string]int)
	for i := 0; i < 2; i++ {
		select {
		case e := <-sub.C():
			got[e.Entity]++
		case <-ctxTimeout(ctx):
		}
	}
	if got["light.b"] != 0 {
		t.Fatal("filter leaked light.b")
	}
}

func TestProperty_CauseChainReachesRoot(t *testing.T) {
	ctx := context.Background()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	p1, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 10))
	p2, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 20, testutil.WithCause(p1)))
	p3, _ := f.store.Append(ctx, testutil.StateChanged("light.a", 30, testutil.WithCause(p2)))

	walk := []uint64{p3}
	cursor := p3
	for cursor != 0 {
		es, err := f.store.Query(ctx, eventstore.QueryOptions{FromPosition: cursor - 1, ToPosition: cursor, Limit: 1})
		if err != nil || len(es) == 0 {
			t.Fatalf("query: %v len=%d", err, len(es))
		}
		cursor = es[0].CausePosition
		if cursor != 0 {
			walk = append(walk, cursor)
		}
	}
	if len(walk) != 3 || walk[len(walk)-1] != p1 {
		t.Fatalf("walk = %v, want root = %d", walk, p1)
	}
}

func ctxTimeout(parent context.Context) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		timer := time.NewTimer(250 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-parent.Done():
		case <-timer.C:
		}
	}()
	return ch
}

func deepEqual(a, b any) bool {
	ma, aok := a.(map[string]any)
	mb, bok := b.(map[string]any)
	if aok != bok {
		return false
	}
	if aok && bok {
		if len(ma) != len(mb) {
			return false
		}
		for k, v := range ma {
			if !deepEqual(v, mb[k]) {
				return false
			}
		}
		return true
	}
	return fmtAny(a) == fmtAny(b)
}

func fmtAny(v any) string { return fmt.Sprintf("%+v", v) }
