//go:build integration

package carport_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/registry"
)

// buildTestDriver compiles cmd/testdriver into a test-temp binary and returns
// its path. Each call is a separate compile; fast enough for a handful of tests.
func buildTestDriver(t *testing.T) string {
	t.Helper()
	outDir := t.TempDir()
	bin := filepath.Join(outDir, "testdriver")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/testdriver")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = findRepoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build testdriver: %v\n%s", err, out)
	}
	return bin
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("repo root not found")
	return ""
}

// waitFor polls cond every 20ms up to d. Returns true if cond() returned true
// before the deadline.
func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

// runScenario sets up a host running one instance with the given TESTDRIVER_MODE
// and waits until `until` returns true, failing the test if it doesn't within 10s.
// Caller gets the running host back for further assertions.
func runScenario(t *testing.T, mode string, until func(*carport.Host) bool) *carport.Host {
	t.Helper()
	bin := buildTestDriver(t)
	f := newStoreFixtureForTest(t)
	reg, err := registry.New(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(reg, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}

	sockDir, err := os.MkdirTemp("", "ghsd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	h, err := carport.New(carport.HostConfig{SocketDir: sockDir},
		f.db, f.store, reg, newTestLogger(), newTestMetrics())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		h.Stop(context.Background())
	})
	if err := h.Start(ctx); err != nil {
		t.Fatal(err)
	}
	params := []byte(`{"TESTDRIVER_MODE":"` + mode + `"}`)
	// Use short timeouts so scenarios resolve within the 10 s test window.
	// The default lifecycle has 15 s health probes which would cause crash-detection
	// tests to time out before the supervisor transitions out of StateRunning.
	lc := carport.LifecycleConfig{
		HandshakeDeadline:       2 * time.Second,
		HealthProbeInterval:     500 * time.Millisecond,
		HealthProbeTimeout:      300 * time.Millisecond,
		HealthFailuresToRestart: 2,
		RestartBackoffInitial:   100 * time.Millisecond,
		RestartBackoffMax:       500 * time.Millisecond,
		RestartBudgetWindow:     time.Minute,
		RestartBudgetMax:        3,
		ShutdownGrace:           time.Second,
	}
	if err := h.RegisterInstanceWithLifecycle(ctx, "test_one", "testdriver", bin, params, lc); err != nil {
		t.Fatal(err)
	}
	if !waitFor(10*time.Second, func() bool { return until(h) }) {
		t.Fatalf("scenario %s: never reached expected state; current=%s", mode, h.InstanceState("test_one"))
	}
	return h
}

func TestIntegration_SupervisorNormalLifecycle(t *testing.T) {
	h := runScenario(t, "normal", func(h *carport.Host) bool {
		return h.InstanceState("test_one") == carport.StateRunning
	})
	h.Stop(context.Background())
	if !waitFor(5*time.Second, func() bool {
		return h.InstanceState("test_one") == carport.StateStopped
	}) {
		t.Fatalf("never stopped; state=%s", h.InstanceState("test_one"))
	}
}

func TestIntegration_CrashAfterHandshake(t *testing.T) {
	runScenario(t, "crash_after_handshake", func(h *carport.Host) bool {
		s := h.InstanceState("test_one")
		return s == carport.StateBackoff || s == carport.StateQuarantined || s == carport.StateSpawning
	})
}

func TestIntegration_CrashMidStream(t *testing.T) {
	runScenario(t, "crash_mid_stream", func(h *carport.Host) bool {
		s := h.InstanceState("test_one")
		return s == carport.StateBackoff || s == carport.StateQuarantined || s == carport.StateSpawning
	})
}

func TestIntegration_BadProtocolVersion(t *testing.T) {
	runScenario(t, "bad_protocol_version", func(h *carport.Host) bool {
		s := h.InstanceState("test_one")
		return s == carport.StateBackoff || s == carport.StateQuarantined
	})
}

func TestIntegration_BadSecret(t *testing.T) {
	runScenario(t, "bad_secret", func(h *carport.Host) bool {
		s := h.InstanceState("test_one")
		return s == carport.StateBackoff || s == carport.StateQuarantined
	})
}

func TestIntegration_SlowHandshake(t *testing.T) {
	// handshake_deadline_ms=2000; driver sleeps 10s.
	runScenario(t, "slow_handshake", func(h *carport.Host) bool {
		s := h.InstanceState("test_one")
		return s == carport.StateBackoff || s == carport.StateQuarantined
	})
}

func TestIntegration_HangOnShutdown(t *testing.T) {
	h := runScenario(t, "hang_on_shutdown", func(h *carport.Host) bool {
		return h.InstanceState("test_one") == carport.StateRunning
	})
	h.Stop(context.Background())
	// shutdown_grace_ms=1000; shutdown RPC hangs forever but supervisor should
	// still force the instance to Stopped via proc.Wait timeout/kill path.
	if !waitFor(8*time.Second, func() bool {
		return h.InstanceState("test_one") == carport.StateStopped
	}) {
		t.Fatalf("hang_on_shutdown never stopped; state=%s", h.InstanceState("test_one"))
	}
}
