package push_test

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/push"
)

func TestSubscriptionStore_RoundTrip(t *testing.T) {
	store := push.NewInMemorySubscriptionStore()
	ctx := context.Background()

	sub := push.Subscription{
		Endpoint: "https://fcm.googleapis.com/test",
		P256DH:   "key",
		Auth:     "secret",
	}

	id, err := store.Register(ctx, "user:alice", sub)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	subs, err := store.List(ctx, "user:alice")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("want 1 subscription, got %d", len(subs))
	}

	if err := store.Unregister(ctx, "user:alice", id); err != nil {
		t.Fatalf("Unregister: %v", err)
	}

	subs, _ = store.List(ctx, "user:alice")
	if len(subs) != 0 {
		t.Fatalf("want 0 after unregister, got %d", len(subs))
	}
}
