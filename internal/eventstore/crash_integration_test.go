//go:build integration

package eventstore_test

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/state"
	"github.com/fdatoo/gohome/internal/storage"
)

func TestCrash_Kill9MidAppendLeavesConsistentDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "gohome.db")

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
	t.Skip("requires helper to write 3 snapshots then test harness to corrupt newest; see spec §7 — scaffold for future")
	_ = os.WriteFile // keep imports
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
