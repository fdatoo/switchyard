package eventstore_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/testutil"
)

func TestSubscribe_LiveDeliversNewEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	sub, err := s.Subscribe(ctx, eventstore.SubscribeOptions{FromPosition: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 10))

	select {
	case got := <-sub.C():
		if got.Entity != "light.a" {
			t.Fatalf("wrong entity %q", got.Entity)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive live event")
	}
}

func TestSubscribe_CatchupReplaysHistory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	for i := 0; i < 5; i++ {
		_, _ = s.Append(ctx, testutil.StateChanged("light.a", uint32(i)))
	}

	sub, err := s.Subscribe(ctx, eventstore.SubscribeOptions{FromPosition: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	got := 0
	timeout := time.After(2 * time.Second)
	for got < 5 {
		select {
		case <-sub.C():
			got++
		case <-timeout:
			t.Fatalf("only received %d of 5 historical events", got)
		}
	}
}

func TestSubscribe_FilterExcludesUnmatchedEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	sub, err := s.Subscribe(ctx, eventstore.SubscribeOptions{
		Filter: eventstore.Filter{Entities: []string{"light.a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	_, _ = s.Append(ctx, testutil.StateChanged("light.b", 5))  // excluded
	_, _ = s.Append(ctx, testutil.StateChanged("light.a", 10)) // included

	select {
	case got := <-sub.C():
		if got.Entity != "light.a" {
			t.Fatalf("filter let through wrong event: %q", got.Entity)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("filter swallowed all events")
	}
}

func TestSubscribe_DurableResumesFromAckedCursor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newStoreFixture(t)
	if err := f.store.Start(ctx); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		_, _ = f.store.Append(ctx, testutil.StateChanged("light.a", uint32(i)))
	}
	sub, err := f.store.Subscribe(ctx, eventstore.SubscribeOptions{Durable: true, Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	var lastSeen uint64
	for i := 0; i < 2; i++ {
		select {
		case e := <-sub.C():
			lastSeen = e.Position
		case <-time.After(2 * time.Second):
			t.Fatal("expected event")
		}
	}
	if err := sub.Ack(lastSeen); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	t.Cleanup(func() { _ = f.store.Close(ctx) })

	// New Store sharing the same DB (simulates daemon restart).
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s2, err := eventstore.Open(context.Background(), eventstore.Config{}, f.db, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close(ctx) })

	sub2, err := s2.Subscribe(ctx, eventstore.SubscribeOptions{Durable: true, Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub2.Close() })

	select {
	case e := <-sub2.C():
		if e.Position <= lastSeen {
			t.Fatalf("resumed at position %d, want > %d", e.Position, lastSeen)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive remaining event")
	}
}
