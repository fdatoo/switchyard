package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fdatoo/switchyard/internal/observability"
)

// SubscribeOptions controls live event delivery and optional durable catchup.
type SubscribeOptions struct {
	FromPosition  uint64
	Filter        Filter
	Durable       bool
	Name          string
	ChannelBuffer int
}

// Subscription is a live stream of events from the store.
type Subscription interface {
	C() <-chan Event
	Ack(position uint64) error
	Close() error
	Stats() SubscriptionStats
}

// SubscriptionStats reports delivery counters and current buffer pressure.
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

	// sendMu serializes Close vs producer sends on `ch`. Producers
	// (dispatch + catchup) take RLock + check isClosed before sending;
	// Close takes Lock to flip the flag and close `ch`. Without this
	// gate, a dispatch goroutine that snapshotted the subscribers list
	// before Close ran would panic on `sub.ch <- e`.
	sendMu   sync.RWMutex
	isClosed atomic.Bool
}

// trySend pushes an event into sub.ch without blocking. Returns true on
// success, false if the channel is full (backpressure) or the subscriber
// has been closed. Safe to call concurrently with Close.
func (sub *subscriber) trySend(e Event) (sent bool, dropped bool) {
	sub.sendMu.RLock()
	defer sub.sendMu.RUnlock()
	if sub.isClosed.Load() {
		return false, false
	}
	select {
	case sub.ch <- e:
		return true, false
	default:
		return false, true
	}
}

// blockingSend pushes an event into sub.ch, blocking until accepted or
// until ctx / sub.closed cancels. Used by catchup where backpressure must
// hold the producer rather than drop events. Returns false on cancel or
// after the subscriber has been closed.
func (sub *subscriber) blockingSend(ctx context.Context, e Event) bool {
	sub.sendMu.RLock()
	defer sub.sendMu.RUnlock()
	if sub.isClosed.Load() {
		return false
	}
	select {
	case sub.ch <- e:
		return true
	case <-ctx.Done():
		return false
	case <-sub.closed:
		return false
	}
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
		// Order matters here. unregister first so dispatch stops adding
		// us to its snapshot. Then signal closed so any in-flight
		// blockingSend on `ch` returns false. Finally take sendMu.Lock
		// to fence against producers that already RLocked and were
		// about to send: they'll either see isClosed=true and bail, or
		// they're already past the check and waiting on the channel,
		// in which case the close(sub.closed) above unblocks them
		// without panicking.
		sub.store.unregisterSubscriber(sub)
		close(sub.closed)
		sub.sendMu.Lock()
		sub.isClosed.Store(true)
		close(sub.ch)
		sub.sendMu.Unlock()
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

// Subscribe returns a live event subscription, optionally after durable catchup.
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

	// Catchup runs async. Doing it synchronously deadlocks when the
	// historical backlog exceeds the channel buffer: the consumer can't
	// drain because we haven't returned the Subscription handle yet, the
	// catchup loop blocks on `sub.ch <- e`, Subscribe never returns. With
	// the goroutine the consumer reads as catchup pushes; live dispatch
	// is held off until catchup completes (registration happens at the
	// end of catchupAndRegister) so order is preserved.
	go func() {
		if err := s.catchupAndRegister(ctx, sub); err != nil {
			s.logger.Warn("subscriber catchup failed; closing", "name", sub.name, "err", err)
			_ = sub.Close()
		}
	}()
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

func (s *Store) catchupAndRegister(ctx context.Context, sub *subscriber) (err error) {
	ctx, span := observability.StartSpan(ctx, "eventstore.SubscriptionCatchup")
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

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
			if !sub.blockingSend(ctx, e) {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return nil // closed
			}
			sub.delivered.Add(1)
			sub.lastSent.Store(e.Position)
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
		sent, dropped := sub.trySend(e)
		if sent {
			sub.delivered.Add(1)
			sub.lastSent.Store(e.Position)
			s.metrics.SubscriptionDelivered.WithLabelValues(sub.name).Inc()
		} else if dropped {
			sub.dropped.Add(1)
			s.metrics.SubscriptionDropped.WithLabelValues(sub.name).Inc()
			s.logger.Warn("subscriber dropped, closing", "name", sub.name)
			// Unregister synchronously so subsequent dispatch calls don't
			// send to a closing channel. Close() unregisters again — no-op.
			s.unregisterSubscriber(sub)
			go func() { _ = sub.Close() }()
		}
		// !sent && !dropped → subscriber already closed; skip silently.
	}
}
