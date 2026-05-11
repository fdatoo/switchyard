package push

import (
	"context"
	"encoding/json"
)

// severityRank maps severity strings to a comparable integer.
var severityRank = map[string]int{
	"debug": 0, "info": 1, "warn": 2, "error": 3, "critical": 4,
}

// Event is a domain event that may be dispatched to push subscribers.
type Event struct {
	Severity string
	Title    string
	Body     string
}

// SentNotification records what was delivered to a subscriber endpoint.
type SentNotification struct {
	Endpoint string
	Title    string
	Body     string
}

// SenderFunc is called once per qualifying subscription. In production it wraps
// webpush.SendNotification; in tests it is a spy.
type SenderFunc func(ctx context.Context, n SentNotification) error

// NotifierConfig holds Notifier configuration.
type NotifierConfig struct {
	MinSeverity     string
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	// Sender is called for each qualifying subscription.
	Sender SenderFunc
}

// Notifier dispatches push notifications for events whose severity meets or
// exceeds the configured minimum.
type Notifier struct {
	store  SubscriptionStore
	config NotifierConfig
}

// NewNotifier creates a Notifier backed by the given subscription store.
func NewNotifier(store SubscriptionStore, config NotifierConfig) *Notifier {
	return &Notifier{store: store, config: config}
}

// HandleEvent checks ev.Severity against the minimum threshold. If the event
// qualifies, it fetches all subscriptions for principalID and calls config.Sender
// for each one.
func (n *Notifier) HandleEvent(ctx context.Context, principalID string, ev Event) {
	if severityRank[ev.Severity] < severityRank[n.config.MinSeverity] {
		return
	}
	subs, err := n.store.List(ctx, principalID)
	if err != nil || len(subs) == 0 {
		return
	}
	payload, _ := json.Marshal(map[string]string{"title": ev.Title, "body": ev.Body})
	for _, s := range subs {
		_ = n.config.Sender(ctx, SentNotification{
			Endpoint: s.Subscription.Endpoint,
			Title:    ev.Title,
			Body:     string(payload),
		})
	}
}
