// Package throttle implements a soft per-IP × per-method failed-auth throttle.
// Backed by auth_attempts table; sweeps rows past the configured window.
package throttle

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrThrottled = errors.New("throttle: too many recent failures")

type Config struct {
	Window    time.Duration
	Threshold uint32
	Block     time.Duration
}

type Throttle struct {
	db  *sql.DB
	cfg Config
}

func New(db *sql.DB, cfg Config) *Throttle { return &Throttle{db: db, cfg: cfg} }

// Check inspects the recent failure count for the bucket; returns
// ErrThrottled if at or above threshold.
func (t *Throttle) Check(ctx context.Context, ip, method string) error {
	bucket := ip + "\x00" + method
	cutoff := time.Now().Add(-t.cfg.Window).Unix()
	var (
		failures uint32
		lastFail int64
	)
	if err := t.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MAX(attempted_at), 0)
		FROM auth_attempts
		WHERE bucket = ? AND succeeded = 0 AND attempted_at >= ?`,
		bucket, cutoff).Scan(&failures, &lastFail); err != nil {
		return err
	}
	if failures >= t.cfg.Threshold {
		if t.cfg.Block == 0 || time.Now().Before(time.Unix(lastFail, 0).Add(t.cfg.Block)) {
			return ErrThrottled
		}
	}
	return nil
}

// Record appends a row reflecting an attempt's outcome.
func (t *Throttle) Record(ctx context.Context, ip, method string, succeeded bool) error {
	bucket := ip + "\x00" + method
	val := 0
	if succeeded {
		val = 1
	}
	_, err := t.db.ExecContext(ctx, `
		INSERT INTO auth_attempts (bucket, attempted_at, succeeded)
		VALUES (?, ?, ?)`,
		bucket, time.Now().Unix(), val)
	return err
}

// Sweep drops rows older than the window cutoff. Run periodically.
func (t *Throttle) Sweep(ctx context.Context) error {
	cutoff := time.Now().Add(-t.cfg.Window).Unix()
	_, err := t.db.ExecContext(ctx, `
		DELETE FROM auth_attempts WHERE attempted_at < ?`, cutoff)
	return err
}
