package eventstore_test

import (
	"context"
	"testing"

	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/state"
	"github.com/fdatoo/gohome/internal/testutil"
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
