//go:build integration

package eventstore_test

import (
	"bufio"
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/storage"
	switchyardtestutil "github.com/fdatoo/switchyard/internal/testutil"
)

func TestCrash_Kill9MidAppendLeavesConsistentDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "switchyard.db")

	binary := buildHelper(t)

	cmd := exec.Command(binary, "-db", dbPath, "-count", "10000")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Wait for READY line.
	ready := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if sc.Text() == "READY" {
				close(ready)
				return
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("helper did not signal READY")
	}

	// Let it append for 50ms, then kill -9.
	time.Sleep(50 * time.Millisecond)
	_ = cmd.Process.Signal(syscall.SIGKILL)
	_ = cmd.Wait()

	// Reopen and verify.
	ctx := context.Background()
	db, err := storage.Open(ctx, storage.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db.Close() }()
	store, err := eventstore.Open(ctx, eventstore.Config{}, db, observability.Init(observability.LogConfig{}), observability.NewMetrics())
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	cache := state.New()
	if err := store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := store.Replay(ctx); err != nil {
		t.Fatalf("replay after crash: %v", err)
	}
	if store.LatestPosition() == 0 {
		t.Fatal("expected some events to have been committed before crash")
	}
	if _, ok := cache.Get("light.x"); !ok {
		t.Fatal("state cache did not restore light.x")
	}
}

func TestCrash_SnapshotCorruptionFallsBack(t *testing.T) {
	t.Run("newest corrupt falls back to previous snapshot", func(t *testing.T) {
		ctx := context.Background()
		dbPath := seedSnapshotDB(t, []uint32{10, 20, 30}, 40)
		db := openTestDB(t, ctx, dbPath)
		corruptSnapshots(t, ctx, db, "position = (SELECT MAX(position) FROM snapshots)")
		if err := db.Close(); err != nil {
			t.Fatalf("close seed db: %v", err)
		}

		metrics, latest := replaySnapshotDB(t, ctx, dbPath)
		if latest != 4 {
			t.Fatalf("LatestPosition = %d, want 4", latest)
		}
		if got := testutil.ToFloat64(metrics.SnapshotCorruption.WithLabelValues("state_cache")); got != 1 {
			t.Fatalf("snapshot corruption total = %v, want 1", got)
		}
		if got := testutil.ToFloat64(metrics.ReplayEventsProcessed); got != 2 {
			t.Fatalf("replayed events = %v, want 2", got)
		}
	})

	t.Run("all snapshots corrupt replays from zero", func(t *testing.T) {
		ctx := context.Background()
		dbPath := seedSnapshotDB(t, []uint32{10, 20, 30}, 40)
		db := openTestDB(t, ctx, dbPath)
		corruptSnapshots(t, ctx, db, "1 = 1")
		if err := db.Close(); err != nil {
			t.Fatalf("close seed db: %v", err)
		}

		metrics, latest := replaySnapshotDB(t, ctx, dbPath)
		if latest != 4 {
			t.Fatalf("LatestPosition = %d, want 4", latest)
		}
		if got := testutil.ToFloat64(metrics.SnapshotCorruption.WithLabelValues("state_cache")); got != 3 {
			t.Fatalf("snapshot corruption total = %v, want 3", got)
		}
		if got := testutil.ToFloat64(metrics.ReplayEventsProcessed); got != 4 {
			t.Fatalf("replayed events = %v, want 4", got)
		}
	})
}

func seedSnapshotDB(t *testing.T, brightnesses []uint32, liveBrightness uint32) string {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "switchyard.db")
	db := openTestDB(t, ctx, dbPath)
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close seed db: %v", err)
		}
	}()

	metrics := observability.NewMetrics()
	store, err := eventstore.Open(ctx, eventstore.Config{}, db, observability.Init(observability.LogConfig{}), metrics)
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}
	defer func() { _ = store.Close(ctx) }()

	cache := state.New()
	if err := store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	for _, brightness := range brightnesses {
		if _, err := store.Append(ctx, switchyardtestutil.StateChanged("light.x", brightness)); err != nil {
			t.Fatalf("append snapshot event: %v", err)
		}
		if _, err := store.SnapshotNow(ctx, "state_cache"); err != nil {
			t.Fatalf("SnapshotNow: %v", err)
		}
	}
	if _, err := store.Append(ctx, switchyardtestutil.StateChanged("light.x", liveBrightness)); err != nil {
		t.Fatalf("append live event: %v", err)
	}
	return dbPath
}

func replaySnapshotDB(t *testing.T, ctx context.Context, dbPath string) (*observability.Metrics, uint64) {
	t.Helper()
	db := openTestDB(t, ctx, dbPath)
	t.Cleanup(func() { _ = db.Close() })

	metrics := observability.NewMetrics()
	store, err := eventstore.Open(ctx, eventstore.Config{}, db, observability.Init(observability.LogConfig{}), metrics)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(ctx) })

	cache := state.New()
	if err := store.RegisterProjector(cache, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if err := store.Replay(ctx); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	assertLightBrightness(t, cache, 40)
	latest := store.LatestPosition()
	if err := store.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := store.Append(ctx, switchyardtestutil.StateChanged("light.x", 50)); err != nil {
		t.Fatalf("append after replay: %v", err)
	}
	assertLightBrightness(t, cache, 50)
	return metrics, latest
}

func corruptSnapshots(t *testing.T, ctx context.Context, db *sql.DB, predicate string) {
	t.Helper()
	result, err := db.ExecContext(ctx,
		`UPDATE snapshots SET state = X'00' WHERE owner = 'state_cache' AND `+predicate,
	)
	if err != nil {
		t.Fatalf("corrupt snapshots: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("corrupt snapshots rows affected: %v", err)
	}
	if rows == 0 {
		t.Fatal("corrupt snapshots affected no rows")
	}
}

func assertLightBrightness(t *testing.T, cache *state.Cache, want uint32) {
	t.Helper()
	got, ok := cache.Get("light.x")
	if !ok {
		t.Fatal("light.x missing from state cache")
	}
	if got.Attributes.GetLight().Brightness != want {
		t.Fatalf("brightness = %d, want %d", got.Attributes.GetLight().Brightness, want)
	}
}

func openTestDB(t *testing.T, ctx context.Context, dbPath string) *sql.DB {
	t.Helper()
	db, err := storage.Open(ctx, storage.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	return db
}

func buildHelper(t *testing.T) string {
	t.Helper()
	// go test sets cwd to the package directory (internal/eventstore).
	// Resolve the module root by going up two levels so that the import path
	// ./internal/eventstore/testdata/crashhelper is valid.
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile: .../internal/eventstore/crash_integration_test.go
	moduleRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	bin := filepath.Join(t.TempDir(), "crashhelper")
	helperPkg := filepath.Join(moduleRoot, "internal", "eventstore", "testdata", "crashhelper")
	cmd := exec.Command("go", "build", "-o", bin, helperPkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	return bin
}
