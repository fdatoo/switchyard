package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const maxUint64 = ^uint64(0)

// Replay restores each sync projector's snapshot, then applies pending
// events in 1000-event batches. Call before Start; not safe after tailer is live.
func (s *Store) Replay(ctx context.Context) error {
	if s.started.Load() {
		return errors.New("Replay: store already started")
	}
	cursor, err := s.restoreProjectors(ctx)
	if err != nil {
		return err
	}
	latest, err := s.loadLatestPosition(ctx)
	if err != nil {
		return err
	}

	for cursor < latest {
		applied, err := s.replayBatch(ctx, cursor, 1000)
		if err != nil {
			return err
		}
		if applied == 0 {
			break
		}
		cursor += applied
		s.metrics.ReplayEventsProcessed.Add(float64(applied))
	}
	return nil
}

func (s *Store) restoreProjectors(ctx context.Context) (uint64, error) {
	minCursor := maxUint64 // max uint64
	for _, reg := range s.projectors {
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return 0, err
		}
		pos, err := reg.p.Restore(ctx, tx)
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("restore %s: %w", reg.p.Name(), err)
		}
		// Only fall back to projection_cursors if Restore loaded a snapshot
		// (pos > 0). A fresh in-memory projector with no snapshot must replay
		// from the beginning regardless of what the cursor table says.
		// (projection_cursors records what was applied in a prior process, not
		// what this in-memory instance has seen.)
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		if pos < minCursor {
			minCursor = pos
		}
	}
	if minCursor == maxUint64 {
		return 0, nil
	}
	return minCursor, nil
}

func (s *Store) loadLatestPosition(ctx context.Context) (uint64, error) {
	var pos sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(position) FROM events`).Scan(&pos)
	if err != nil {
		return 0, err
	}
	if !pos.Valid {
		return 0, nil
	}
	return uint64(pos.Int64), nil
}

func (s *Store) replayBatch(ctx context.Context, after uint64, limit int) (uint64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT position, ts, kind, entity, source, correlation_id, cause_position, payload
		FROM events WHERE position > ? ORDER BY position LIMIT ?`, after, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close() //nolint:errcheck

	var batch []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return 0, err
		}
		batch = append(batch, e)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}

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

	for _, e := range batch {
		for _, reg := range s.projectors {
			if reg.mode != ProjectorModeSync {
				continue
			}
			skipped, err := s.isSkipped(ctx, tx, e.Position, reg.p.Name())
			if err != nil {
				return 0, err
			}
			if skipped {
				s.logger.Warn("skipping event per skipped_events table",
					"position", e.Position, "projector", reg.p.Name())
				continue
			}
			if err := reg.p.Apply(ctx, tx, e); err != nil {
				return 0, fmt.Errorf("replay projector %s at position %d: %w",
					reg.p.Name(), e.Position, err)
			}
		}
	}

	// Advance cursors to last event in batch.
	last := batch[len(batch)-1].Position
	for _, reg := range s.projectors {
		if reg.mode != ProjectorModeSync {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO projection_cursors (name, position, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`,
			reg.p.Name(), last, time.Now().UnixNano()); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true

	// Promote in-memory projector state after transaction commits.
	for _, reg := range s.projectors {
		if pc, ok := reg.p.(PostCommit); ok {
			pc.Promote()
		}
	}
	return uint64(len(batch)), nil
}

func (s *Store) isSkipped(ctx context.Context, tx *sql.Tx, pos uint64, projector string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skipped_events WHERE position = ? AND projector = ?`,
		pos, projector,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
