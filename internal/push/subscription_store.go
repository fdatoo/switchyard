package push

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// Subscription holds Web Push credentials for a single browser endpoint.
type Subscription struct {
	Endpoint  string
	P256DH    string
	Auth      string
	UserAgent string
}

// IndexedSubscription is a Subscription paired with its opaque store ID.
type IndexedSubscription struct {
	ID           string
	Subscription Subscription
}

// SubscriptionStore manages push subscriptions per principal.
type SubscriptionStore interface {
	Register(ctx context.Context, principalID string, sub Subscription) (id string, err error)
	List(ctx context.Context, principalID string) ([]IndexedSubscription, error)
	Unregister(ctx context.Context, principalID string, id string) error
}

type inMemorySubscriptionStore struct {
	mu   sync.RWMutex
	data map[string][]IndexedSubscription // principalID → subs
}

// NewInMemorySubscriptionStore returns an in-memory SubscriptionStore suitable
// for tests and single-node deployments.
func NewInMemorySubscriptionStore() SubscriptionStore {
	return &inMemorySubscriptionStore{data: make(map[string][]IndexedSubscription)}
}

func (s *inMemorySubscriptionStore) Register(_ context.Context, principalID string, sub Subscription) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := uuid.NewString()
	s.data[principalID] = append(s.data[principalID], IndexedSubscription{ID: id, Subscription: sub})
	return id, nil
}

func (s *inMemorySubscriptionStore) List(_ context.Context, principalID string) ([]IndexedSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[principalID], nil
}

func (s *inMemorySubscriptionStore) Unregister(_ context.Context, principalID string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	subs := s.data[principalID]
	for i, entry := range subs {
		if entry.ID == id {
			s.data[principalID] = append(subs[:i], subs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found for %q", id, principalID)
}
