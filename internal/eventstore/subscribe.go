package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type SubscribeOptions struct {
	FromPosition  uint64
	Filter        Filter
	Durable       bool
	Name          string
	ChannelBuffer int
}

type Subscription interface {
	C() <-chan Event
	Ack(position uint64) error
	Close() error
	Stats() SubscriptionStats
}

type SubscriptionStats struct {
	Delivered uint64
	Dropped   uint64
	LagEvents uint64
	Buffered  int
}

type subscriber struct {
	name      string
	filter    Filter
	ch        chan Event
	delivered atomic.Uint64
	dropped   atomic.Uint64
	lastSent  atomic.Uint64
	store     *Store
	durable   bool

	closeOnce sync.Once
	closed    chan struct{}
}

func (sub *subscriber) C() <-chan Event { return sub.ch }

func (sub *subscriber) Ack(position uint64) error {
	if !sub.durable {
		return nil
	}
	return sub.store.persistCursor(context.Background(), sub.name, position)
}

func (sub *subscriber) Close() error {
	sub.closeOnce.Do(func() {
		sub.store.unregisterSubscriber(sub)
		close(sub.closed)
		close(sub.ch)
	})
	return nil
}

func (sub *subscriber) Stats() SubscriptionStats {
	return SubscriptionStats{
		Delivered: sub.delivered.Load(),
		Dropped:   sub.dropped.Load(),
		LagEvents: 0,
		Buffered:  len(sub.ch),
	}
}

func (s *Store) Subscribe(ctx context.Context, opts SubscribeOptions) (Subscription, error) {
	if !s.started.Load() {
		return nil, errors.New("Subscribe: store not started")
	}
	if opts.Durable && opts.Name == "" {
		return nil, errors.New("Subscribe: Durable requires Name")
	}

	buf := opts.ChannelBuffer
	if buf <= 0 {
		buf = s.cfg.MaxSubscriberBuffer
	}
	name := opts.Name
	if name == "" {
		name = "anon"
	}

	fromPos := opts.FromPosition
	if opts.Durable {
		cursor, err := s.loadDurableCursor(ctx, name, opts.FromPosition)
		if err != nil {
			return nil, err
		}
		fromPos = cursor
	}

	sub := &subscriber{
		name:    name,
		filter:  opts.Filter,
		ch:      make(chan Event, buf),
		store:   s,
		closed:  make(chan struct{}),
		durable: opts.Durable,
	}
	sub.lastSent.Store(fromPos)

	if err := s.catchupAndRegister(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *Store) loadDurableCursor(ctx context.Context, name string, fallback uint64) (uint64, error) {
	var cursor int64
	err := s.db.QueryRowContext(ctx,
		`SELECT cursor FROM event_subscriptions WHERE name = ?`, name,
	).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UnixNano()
		_, ierr := s.db.ExecContext(ctx, `
            INSERT INTO event_subscriptions (name, cursor, created_at, last_active)
            VALUES (?, ?, ?, ?)`,
			name, int64(fallback), now, now,
		)
		if ierr != nil {
			return 0, fmt.Errorf("insert durable subscription: %w", ierr)
		}
		return fallback, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load durable cursor: %w", err)
	}
	return uint64(cursor), nil
}

func (s *Store) persistCursor(ctx context.Context, name string, position uint64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE event_subscriptions SET cursor = ?, last_active = ? WHERE name = ?`,
		int64(position), time.Now().UnixNano(), name,
	)
	if err != nil {
		return fmt.Errorf("persist cursor %s: %w", name, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("persist cursor %s: subscription not found", name)
	}
	return nil
}

func (s *Store) catchupAndRegister(ctx context.Context, sub *subscriber) error {
	for {
		s.mu.RLock()
		target := s.latestPosition
		s.mu.RUnlock()

		cursor := sub.lastSent.Load()
		if cursor >= target {
			s.mu.Lock()
			if s.latestPosition > cursor {
				s.mu.Unlock()
				continue
			}
			s.subs = append(s.subs, sub)
			s.metrics.SubscriptionActive.WithLabelValues(sub.name).Set(1)
			s.mu.Unlock()
			return nil
		}

		evts, err := s.Query(ctx, QueryOptions{
			FromPosition: cursor,
			ToPosition:   target,
			Filter:       sub.filter,
			Limit:        1000,
		})
		if err != nil {
			return err
		}
		for _, e := range evts {
			select {
			case sub.ch <- e:
				sub.delivered.Add(1)
				sub.lastSent.Store(e.Position)
			case <-ctx.Done():
				return ctx.Err()
			case <-sub.closed:
				return nil
			}
		}
		if len(evts) == 0 {
			sub.lastSent.Store(target)
		}
	}
}

func (s *Store) unregisterSubscriber(target *subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.subs[:0]
	for _, sub := range s.subs {
		if sub != target {
			out = append(out, sub)
		}
	}
	s.subs = out
	s.metrics.SubscriptionActive.WithLabelValues(target.name).Set(0)
}

func (s *Store) dispatch(e Event) {
	s.mu.RLock()
	subs := make([]*subscriber, len(s.subs))
	copy(subs, s.subs)
	s.mu.RUnlock()

	for _, sub := range subs {
		if !sub.filter.Matches(e) {
			continue
		}
		// Skip events already delivered via catchupAndRegister. This guards
		// against the startup race where the tailer cursor starts below a
		// subscriber's catchup watermark and would otherwise re-dispatch history.
		if sub.lastSent.Load() >= e.Position {
			continue
		}
		select {
		case sub.ch <- e:
			sub.delivered.Add(1)
			sub.lastSent.Store(e.Position)
			s.metrics.SubscriptionDelivered.WithLabelValues(sub.name).Inc()
		default:
			sub.dropped.Add(1)
			s.metrics.SubscriptionDropped.WithLabelValues(sub.name).Inc()
			s.logger.Warn("subscriber dropped, closing", "name", sub.name)
			// Unregister synchronously so subsequent dispatch calls don't send to a
			// closing channel. Close() will call unregisterSubscriber again (no-op).
			s.unregisterSubscriber(sub)
			go func() { _ = sub.Close() }()
		}
	}
}
