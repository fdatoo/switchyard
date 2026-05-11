package push_test

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/push"
)

func TestNotifier_SendsOnHighSeverity(t *testing.T) {
	store := push.NewInMemorySubscriptionStore()
	ctx := context.Background()
	_, _ = store.Register(ctx, "user:alice", push.Subscription{
		Endpoint: "https://example.com/push/test",
		P256DH:   "BAAAAA==",
		Auth:     "secret",
	})

	var sent []push.SentNotification
	notifier := push.NewNotifier(store, push.NotifierConfig{
		MinSeverity:     "warn",
		VAPIDPublicKey:  "BNullPublicKey",
		VAPIDPrivateKey: "NullPrivateKey",
		Sender: func(_ context.Context, n push.SentNotification) error {
			sent = append(sent, n)
			return nil
		},
	})

	// below threshold — should not send
	notifier.HandleEvent(ctx, "user:alice", push.Event{Severity: "info", Title: "quiet", Body: "ignored"})
	if len(sent) != 0 {
		t.Fatalf("want 0 sends for info, got %d", len(sent))
	}

	// at threshold — should send
	notifier.HandleEvent(ctx, "user:alice", push.Event{Severity: "warn", Title: "Motion", Body: "Front door"})
	if len(sent) != 1 {
		t.Fatalf("want 1 send for warn, got %d", len(sent))
	}
	if sent[0].Title != "Motion" {
		t.Fatalf("want title Motion, got %q", sent[0].Title)
	}
}
