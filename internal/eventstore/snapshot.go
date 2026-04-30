package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SnapshotNow forces an immediate snapshot for the named owner projector.
// Pass "" to snapshot every registered projector sequentially.
func (s *Store) SnapshotNow(ctx context.Context, owner string) (uint64, error) {
	targets := s.projectors
	if owner != "" {
		found := false
		for _, reg := range s.projectors {
			if reg.p.Name() == owner {
				targets = []projectorReg{reg}
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("snapshot: unknown owner %q", owner)
		}
	}

	var lastPos uint64
	for _, reg := range targets {
		pos, err := s.runSnapshot(ctx, reg.p)
		if err != nil {
			return 0, err
		}
		if pos > lastPos {
			lastPos = pos
		}
	}
	return lastPos, nil
}

func (s *Store) runSnapshot(ctx context.Context, p Projector) (uint64, error) {
	start := time.Now()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := p.Snapshot(ctx, tx); err != nil {
		return 0, fmt.Errorf("snapshot %s: %w", p.Name(), err)
	}

	// Prune snapshots beyond retain count.
	retain := s.cfg.SnapshotRetainPerOwner
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM snapshots WHERE owner = ? AND position NOT IN (
			SELECT position FROM snapshots WHERE owner = ? ORDER BY position DESC LIMIT ?
		)`, p.Name(), p.Name(), retain,
	); err != nil {
		return 0, fmt.Errorf("prune snapshots: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true

	s.metrics.SnapshotDuration.WithLabelValues(p.Name()).Observe(time.Since(start).Seconds())

	// Report the row just inserted.
	var pos int64
	err = s.db.QueryRowContext(ctx,
		`SELECT position FROM snapshots WHERE owner = ? ORDER BY position DESC LIMIT 1`,
		p.Name(),
	).Scan(&pos)
	if errors.Is(err, sql.ErrNoRows) {
		// Projector is no-snapshot (e.g. registry) — return latest position.
		return s.LatestPosition(), nil
	}
	if err != nil {
		return 0, err
	}
	s.metrics.SnapshotLastPos.WithLabelValues(p.Name()).Set(float64(pos))
	return uint64(pos), nil
}

type snapshotEntry struct {
	projector Projector
	lastRun   time.Time
	lastPos   uint64
}

// startSnapshotter runs a background goroutine checking snapshot cadence every minute.
func (s *Store) startSnapshotter(ctx context.Context) {
	entries := make([]*snapshotEntry, 0, len(s.projectors))
	for _, reg := range s.projectors {
		entries = append(entries, &snapshotEntry{projector: reg.p, lastRun: time.Now()})
	}
	s.bgWG.Add(1)
	go func() {
		defer s.bgWG.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, e := range entries {
					current := s.LatestPosition()
					eventsSince := current - e.lastPos
					timeSince := time.Since(e.lastRun)
					if eventsSince >= uint64(s.cfg.SnapshotEveryEvents) || timeSince >= s.cfg.SnapshotEveryPeriod {
						if pos, err := s.runSnapshot(ctx, e.projector); err == nil {
							e.lastRun = time.Now()
							e.lastPos = pos
						} else {
							s.logger.Error("snapshot failed", "owner", e.projector.Name(), "err", err)
						}
					}
				}
			}
		}
	}()
}
