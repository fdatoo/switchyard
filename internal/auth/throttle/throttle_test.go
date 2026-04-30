package throttle_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/auth/identity"
	"github.com/fdatoo/gohome/internal/auth/throttle"
	"github.com/fdatoo/gohome/internal/testutil"
)

func setupThrottleDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testutil.NewTestDB(t)
	_, err := identity.New(context.Background(), db)
	require.NoError(t, err)
	return db
}

func TestThrottle_BlockAfterThreshold(t *testing.T) {
	db := setupThrottleDB(t)
	th := throttle.New(db, throttle.Config{Window: time.Minute, Threshold: 3, Block: time.Minute})
	ctx := context.Background()
	require.NoError(t, th.Check(ctx, "10.0.0.1", "password"))
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", false))
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", false))
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", false))
	require.ErrorIs(t, th.Check(ctx, "10.0.0.1", "password"), throttle.ErrThrottled)
}

func TestThrottle_DifferentMethodIndependent(t *testing.T) {
	db := setupThrottleDB(t)
	th := throttle.New(db, throttle.Config{Window: time.Minute, Threshold: 1, Block: time.Minute})
	ctx := context.Background()
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", false))
	require.ErrorIs(t, th.Check(ctx, "10.0.0.1", "password"), throttle.ErrThrottled)
	require.NoError(t, th.Check(ctx, "10.0.0.1", "passkey"))
}

func TestThrottle_SuccessDoesNotClearCounter(t *testing.T) {
	db := setupThrottleDB(t)
	th := throttle.New(db, throttle.Config{Window: time.Minute, Threshold: 2, Block: time.Minute})
	ctx := context.Background()
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", false))
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", true))  // success after one fail
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", false)) // and one more failure
	require.ErrorIs(t, th.Check(ctx, "10.0.0.1", "password"), throttle.ErrThrottled)
}

func TestThrottle_SweepClearsOldRows(t *testing.T) {
	db := setupThrottleDB(t)
	// Use a 1-second window for throttling, but we need to account for Unix timestamp
	// integer truncation: we must wait > 2 seconds for a row to become eligible for sweep.
	th := throttle.New(db, throttle.Config{Window: time.Second, Threshold: 1, Block: 0})
	ctx := context.Background()
	require.NoError(t, th.Record(ctx, "10.0.0.1", "password", false))
	require.ErrorIs(t, th.Check(ctx, "10.0.0.1", "password"), throttle.ErrThrottled)
	time.Sleep(2100 * time.Millisecond)
	require.NoError(t, th.Sweep(ctx))
	require.NoError(t, th.Check(ctx, "10.0.0.1", "password"))
}
