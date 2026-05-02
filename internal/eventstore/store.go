package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/observability"
)

type Config struct {
	SnapshotEveryEvents    int
	SnapshotEveryPeriod    time.Duration
	MaxSubscriberBuffer    int
	SnapshotRetainPerOwner int
}

func (c *Config) withDefaults() {
	if c.SnapshotEveryEvents == 0 {
		c.SnapshotEveryEvents = 10_000
	}
	if c.SnapshotEveryPeriod == 0 {
		c.SnapshotEveryPeriod = time.Hour
	}
	if c.MaxSubscriberBuffer == 0 {
		c.MaxSubscriberBuffer = 256
	}
	if c.SnapshotRetainPerOwner == 0 {
		c.SnapshotRetainPerOwner = 3
	}
}

type projectorReg struct {
	p    Projector
	mode ProjectorMode
}

type Store struct {
	cfg     Config
	db      *sql.DB
	logger  *slog.Logger
	metrics *observability.Metrics

	projectors []projectorReg

	mu             sync.RWMutex
	latestPosition uint64
	cond           *sync.Cond
	subs           []*subscriber // populated in later tasks
	started        atomic.Bool
	cancel         context.CancelFunc // cancels the tailer goroutine
	bgWG           sync.WaitGroup     // tailer + snapshotter lifetime
}

// Open constructs a Store around an already-migrated *sql.DB.
// Callers must RegisterProjector and Start before Append.
func Open(ctx context.Context, cfg Config, db *sql.DB, logger *slog.Logger, metrics *observability.Metrics) (*Store, error) {
	cfg.withDefaults()
	s := &Store{
		cfg:     cfg,
		db:      db,
		logger:  logger,
		metrics: metrics,
	}
	s.cond = sync.NewCond(&s.mu)

	var latest sql.NullInt64
	err := db.QueryRowContext(ctx, "SELECT MAX(position) FROM events").Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load latest position: %w", err)
	}
	if latest.Valid {
		s.latestPosition = uint64(latest.Int64)
	}
	return s, nil
}

func (s *Store) RegisterProjector(p Projector, mode ProjectorMode) error {
	if s.started.Load() {
		return errors.New("RegisterProjector: already started")
	}
	s.projectors = append(s.projectors, projectorReg{p: p, mode: mode})
	return nil
}

func (s *Store) LatestPosition() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestPosition
}

// Append writes a single event. Sync projectors run in the same transaction.
// Returns the assigned position.
func (s *Store) Append(ctx context.Context, e Event) (uint64, error) {
	if e.Kind == "" {
		return 0, errors.New("Append: Kind required")
	}
	if e.Source == "" {
		return 0, errors.New("Append: Source required")
	}
	if e.Payload == nil {
		return 0, errors.New("Append: Payload required")
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	if e.CorrelationID == uuid.Nil {
		e.CorrelationID = uuid.New()
	}

	payload, err := proto.Marshal(e.Payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	start := time.Now()
	defer func() { s.metrics.AppendDuration.Observe(time.Since(start).Seconds()) }()

	var position uint64
	if err := s.withRetry(ctx, func() error {
		return s.appendTx(ctx, e, payload, &position)
	}); err != nil {
		return 0, err
	}

	s.mu.Lock()
	if position > s.latestPosition {
		s.latestPosition = position
	}
	s.mu.Unlock()
	s.cond.Broadcast()

	s.metrics.EventsAppended.WithLabelValues(e.Kind).Inc()
	return position, nil
}

// AppendAuth wraps e in a Payload_AuthEvent and appends it.
func (s *Store) AppendAuth(ctx context.Context, e *eventv1.AuthEvent) error {
	_, err := s.Append(ctx, Event{
		Kind:    "auth_event",
		Source:  "auth.audit",
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AuthEvent{AuthEvent: e}},
	})
	return err
}

func (s *Store) appendTx(ctx context.Context, e Event, payload []byte, outPos *uint64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			for _, reg := range s.projectors {
				if d, ok := reg.p.(Discarder); ok {
					d.Discard()
				}
			}
			_ = tx.Rollback()
		}
	}()

	var corrBytes []byte
	if e.CorrelationID != uuid.Nil {
		b, _ := e.CorrelationID.MarshalBinary()
		corrBytes = b
	}
	var causePos sql.NullInt64
	if e.CausePosition > 0 {
		causePos = sql.NullInt64{Int64: int64(e.CausePosition), Valid: true}
	}
	var entity sql.NullString
	if e.Entity != "" {
		entity = sql.NullString{String: e.Entity, Valid: true}
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO events (ts, kind, entity, source, correlation_id, cause_position, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp.UnixNano(), e.Kind, entity, e.Source, corrBytes, causePos, payload,
	)
	if err != nil {
		s.metrics.AppendFailures.WithLabelValues("insert").Inc()
		return fmt.Errorf("insert event: %w", err)
	}

	pos, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	e.Position = uint64(pos)
	*outPos = e.Position

	for _, reg := range s.projectors {
		if reg.mode != ProjectorModeSync {
			continue
		}
		projStart := time.Now()
		if err := reg.p.Apply(ctx, tx, e); err != nil {
			s.metrics.ProjectorFailures.WithLabelValues(reg.p.Name(), "sync").Inc()
			s.metrics.AppendFailures.WithLabelValues("projector").Inc()
			return fmt.Errorf("projector %s apply: %w", reg.p.Name(), err)
		}
		s.metrics.ProjectorApplyDuration.WithLabelValues(reg.p.Name(), "sync").Observe(time.Since(projStart).Seconds())

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO projection_cursors (name, position, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`,
			reg.p.Name(), e.Position, time.Now().UnixNano(),
		); err != nil {
			return fmt.Errorf("advance cursor %s: %w", reg.p.Name(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		s.metrics.AppendFailures.WithLabelValues("commit").Inc()
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	for _, reg := range s.projectors {
		if pc, ok := reg.p.(PostCommit); ok {
			pc.Promote()
		}
	}
	return nil
}

func (s *Store) withRetry(ctx context.Context, fn func() error) error {
	backoffs := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}
	var err error
	for attempt := 0; attempt <= len(backoffs); attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isSQLiteBusy(err) {
			return err
		}
		s.metrics.AppendRetries.Inc()
		if attempt == len(backoffs) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffs[attempt]):
		}
	}
	return err
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "(5)")
}

// AppendBatch writes multiple events atomically. All-or-nothing.
func (s *Store) AppendBatch(ctx context.Context, events []Event) ([]uint64, error) {
	if len(events) == 0 {
		return nil, nil
	}
	positions := make([]uint64, len(events))

	err := s.withRetry(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				for _, reg := range s.projectors {
					if d, ok := reg.p.(Discarder); ok {
						d.Discard()
					}
				}
				_ = tx.Rollback()
			}
		}()

		for i := range events {
			e := &events[i]
			if e.Kind == "" || e.Source == "" || e.Payload == nil {
				return errors.New("AppendBatch: invalid event")
			}
			if e.Timestamp.IsZero() {
				e.Timestamp = time.Now()
			}
			if e.CorrelationID == uuid.Nil {
				e.CorrelationID = uuid.New()
			}
			payload, err := proto.Marshal(e.Payload)
			if err != nil {
				return fmt.Errorf("marshal[%d]: %w", i, err)
			}
			var corr []byte
			if e.CorrelationID != uuid.Nil {
				corr, _ = e.CorrelationID.MarshalBinary()
			}
			var cause sql.NullInt64
			if e.CausePosition > 0 {
				cause = sql.NullInt64{Int64: int64(e.CausePosition), Valid: true}
			}
			var ent sql.NullString
			if e.Entity != "" {
				ent = sql.NullString{String: e.Entity, Valid: true}
			}
			res, err := tx.ExecContext(ctx, `
				INSERT INTO events (ts, kind, entity, source, correlation_id, cause_position, payload)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				e.Timestamp.UnixNano(), e.Kind, ent, e.Source, corr, cause, payload,
			)
			if err != nil {
				return err
			}
			pos, err := res.LastInsertId()
			if err != nil {
				return err
			}
			e.Position = uint64(pos)
			positions[i] = e.Position

			for _, reg := range s.projectors {
				if reg.mode != ProjectorModeSync {
					continue
				}
				if err := reg.p.Apply(ctx, tx, *e); err != nil {
					return fmt.Errorf("projector %s: %w", reg.p.Name(), err)
				}
			}
		}

		for _, reg := range s.projectors {
			if reg.mode != ProjectorModeSync {
				continue
			}
			last := events[len(events)-1].Position
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO projection_cursors (name, position, updated_at) VALUES (?, ?, ?)
				ON CONFLICT(name) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`,
				reg.p.Name(), last, time.Now().UnixNano(),
			); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		for _, reg := range s.projectors {
			if pc, ok := reg.p.(PostCommit); ok {
				pc.Promote()
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.latestPosition = events[len(events)-1].Position
	s.mu.Unlock()
	s.cond.Broadcast()

	for _, e := range events {
		s.metrics.EventsAppended.WithLabelValues(e.Kind).Inc()
	}
	return positions, nil
}

// Close releases the store.
func (s *Store) Close(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.cond.Broadcast()
	s.bgWG.Wait()
	s.mu.Lock()
	subs := make([]*subscriber, len(s.subs))
	copy(subs, s.subs)
	s.subs = s.subs[:0]
	s.mu.Unlock()
	for _, sub := range subs {
		_ = sub.Close()
	}
	return nil
}
