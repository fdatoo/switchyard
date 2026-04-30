package state_test

import (
	"context"
	"sync"
	"testing"
	"time"

	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/state"
)

func mkStateChanged(entity string, on bool, brightness uint32) eventstore.Event {
	return eventstore.Event{
		Position:  1,
		Timestamp: time.Now(),
		Kind:      "state_changed",
		Entity:    entity,
		Source:    "test",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_StateChanged{
			StateChanged: &eventv1.StateChanged{
				Attributes: &entityv1.Attributes{
					Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{On: on, Brightness: brightness}},
				},
			},
		}},
	}
}

func TestCache_EmptyCacheHasZeroLen(t *testing.T) {
	c := state.New()
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0", c.Len())
	}
	if _, ok := c.Get("light.lr"); ok {
		t.Fatal("empty cache returned a state")
	}
}

func TestCache_ApplyStateChangedStoresState(t *testing.T) {
	c := state.New()
	if err := c.Apply(context.Background(), nil, mkStateChanged("light.lr", true, 100)); err != nil {
		t.Fatal(err)
	}
	// IMPORTANT: in production, Apply builds a pending snapshot; promotion happens post-commit.
	// For this unit test, Promote exposes the post-commit swap explicitly.
	c.Promote()

	s, ok := c.Get("light.lr")
	if !ok {
		t.Fatal("entity missing")
	}
	if s.Attributes.GetLight().Brightness != 100 {
		t.Fatalf("brightness = %d, want 100", s.Attributes.GetLight().Brightness)
	}
}

func TestCache_ViewIsStableDuringWrites(t *testing.T) {
	c := state.New()
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.a", true, 50))
	c.Promote()
	snap := c.View()

	// Apply + promote new events.
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.b", true, 90))
	c.Promote()
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.a", false, 0))
	c.Promote()

	// Original snapshot should still reflect old state.
	if _, ok := snap.Get("light.b"); ok {
		t.Fatal("old snapshot saw later write to light.b")
	}
	if got, ok := snap.Get("light.a"); !ok || got.Attributes.GetLight().Brightness != 50 {
		t.Fatal("old snapshot mutated light.a")
	}
}

func TestCache_DiscardDropsPending(t *testing.T) {
	c := state.New()
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.lr", true, 77))
	// Discard instead of Promote — pending must be gone.
	c.Discard()
	if _, ok := c.Get("light.lr"); ok {
		t.Fatal("entity visible after Discard")
	}
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0 after Discard", c.Len())
	}
}

func TestCache_PromoteNilPendingIsNoop(t *testing.T) {
	c := state.New()
	c.Promote() // must not panic when pending == nil
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0", c.Len())
	}
}

func TestCache_ApplyEntityRegisteredSeeds(t *testing.T) {
	c := state.New()
	evt := eventstore.Event{
		Position:  1,
		Timestamp: time.Now(),
		Kind:      "entity_registered",
		Entity:    "light.new",
		Source:    "driver:x",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				Capabilities: &entityv1.Attributes{
					Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{Brightness: 42}},
				},
			},
		}},
	}
	if err := c.Apply(context.Background(), nil, evt); err != nil {
		t.Fatal(err)
	}
	c.Promote()
	s, ok := c.Get("light.new")
	if !ok {
		t.Fatal("entity not seeded by EntityRegistered")
	}
	if s.Attributes.GetLight().Brightness != 42 {
		t.Fatalf("brightness = %d, want 42", s.Attributes.GetLight().Brightness)
	}
}

func TestCache_ApplyEntityRegisteredDoesNotOverwrite(t *testing.T) {
	c := state.New()
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.lr", true, 100))
	c.Promote()
	// entity_registered should not overwrite existing state.
	evt := eventstore.Event{
		Position:  2,
		Timestamp: time.Now(),
		Kind:      "entity_registered",
		Entity:    "light.lr",
		Source:    "driver:x",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityRegistered{
			EntityRegistered: &eventv1.EntityRegistered{
				Capabilities: &entityv1.Attributes{
					Kind: &entityv1.Attributes_Light{Light: &entityv1.Light{Brightness: 1}},
				},
			},
		}},
	}
	_ = c.Apply(context.Background(), nil, evt)
	c.Promote()
	s, _ := c.Get("light.lr")
	if s.Attributes.GetLight().Brightness != 100 {
		t.Fatalf("entity_registered overwrote existing: brightness = %d", s.Attributes.GetLight().Brightness)
	}
}

func TestCache_ApplyEntityUnregisteredRemoves(t *testing.T) {
	c := state.New()
	_ = c.Apply(context.Background(), nil, mkStateChanged("light.lr", true, 100))
	c.Promote()
	evt := eventstore.Event{
		Position:  2,
		Timestamp: time.Now(),
		Kind:      "entity_unregistered",
		Entity:    "light.lr",
		Source:    "driver:x",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_EntityUnregistered{
			EntityUnregistered: &eventv1.EntityUnregistered{},
		}},
	}
	_ = c.Apply(context.Background(), nil, evt)
	c.Promote()
	if _, ok := c.Get("light.lr"); ok {
		t.Fatal("entity still present after EntityUnregistered")
	}
}

func TestCache_ConcurrentReadersAreSafe(t *testing.T) {
	c := state.New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = c.Apply(context.Background(), nil, mkStateChanged("light.x", true, uint32(i)))
			c.Promote()
		}(i)
	}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.Get("light.x")
		}()
	}
	wg.Wait()
}
