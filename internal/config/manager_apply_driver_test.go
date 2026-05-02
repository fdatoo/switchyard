//go:build integration

package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

type recordingCarport struct {
	mu           sync.Mutex
	registered   []registeredInst
	unregistered []string
}

type registeredInst struct {
	id, driverName, binary string
	enabled                bool
	lifecycle              carport.LifecycleConfig
}

func (f *recordingCarport) RegisterInstance(_ context.Context, id, driverName, binary string, _ []byte, enabled bool, lc carport.LifecycleConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registered = append(f.registered, registeredInst{id, driverName, binary, enabled, lc})
	return nil
}
func (f *recordingCarport) UnregisterInstance(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unregistered = append(f.unregistered, id)
	return nil
}

type nopStore struct{}

func (nopStore) Append(_ context.Context, _ eventstore.Event) (uint64, error) { return 0, nil }

const fakeDriverManifest = `
extends "switchyard:driver"
const name = "fake"
const version = "1.0"
produces = new { "light" }
lifecycleDefaults {
  restartBudgetMax = 7
}
class FakeInstance extends Instance {
  driverName = name
}
`

const fakeDriverManifestNoLifecycle = `
extends "switchyard:driver"
const name = "fake"
const version = "1.0"
produces = new { "light" }
class FakeInstance extends Instance {
  driverName = name
}
`

func writeMainPkl(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "main.pkl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManagerApply_ResolvesBinaryFromRegistry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	driversRoot := t.TempDir()
	configDir := t.TempDir()
	writeManifest(t, driversRoot, "fake", fakeDriverManifest)
	writeMainPkl(t, configDir, `
amends "switchyard:config"
import "switchyard:carport" as carport
import "driver:fake" as fake

driverInstances = new {
  new fake.FakeInstance {
    id = "fake_one"
  }
}
`)

	cp := &recordingCarport{}
	mgr, err := NewManager(ctx, configDir, driversRoot, nopStore{}, cp)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Apply(ctx, false); err != nil {
		t.Fatal(err)
	}
	if len(cp.registered) != 1 {
		t.Fatalf("registered = %d, want 1", len(cp.registered))
	}
	got := cp.registered[0]
	wantBin := filepath.Join(driversRoot, "fake", "fake-driver")
	if got.binary != wantBin {
		t.Errorf("binary = %q, want %q", got.binary, wantBin)
	}
	if got.driverName != "fake" {
		t.Errorf("driverName = %q, want %q", got.driverName, "fake")
	}
	if got.lifecycle.RestartBudgetMax != 7 {
		t.Errorf("restartBudgetMax = %d, want 7 (manifest default)", got.lifecycle.RestartBudgetMax)
	}
	// Defaults left alone by manifest fall through to Go defaults.
	if got.lifecycle.HandshakeDeadline != 5*time.Second {
		t.Errorf("handshakeDeadline = %v, want 5s (Go default)", got.lifecycle.HandshakeDeadline)
	}
}

func TestManagerApply_PerInstanceLifecycleOverride(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	driversRoot := t.TempDir()
	configDir := t.TempDir()
	writeManifest(t, driversRoot, "fake", fakeDriverManifest)
	writeMainPkl(t, configDir, `
amends "switchyard:config"
import "switchyard:carport" as carport
import "driver:fake" as fake

driverInstances = new {
  new fake.FakeInstance {
    id = "fake_one"
    lifecycle = new carport.LifecycleOverride { restartBudgetMax = 99 }
  }
}
`)

	cp := &recordingCarport{}
	mgr, err := NewManager(ctx, configDir, driversRoot, nopStore{}, cp)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Apply(ctx, false); err != nil {
		t.Fatal(err)
	}
	if cp.registered[0].lifecycle.RestartBudgetMax != 99 {
		t.Errorf("restartBudgetMax = %d, want 99 (per-instance override wins)", cp.registered[0].lifecycle.RestartBudgetMax)
	}
}

func TestManagerApply_DisabledInstanceNotRegistered(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	driversRoot := t.TempDir()
	configDir := t.TempDir()
	writeManifest(t, driversRoot, "fake", fakeDriverManifestNoLifecycle)
	writeMainPkl(t, configDir, `
amends "switchyard:config"
import "switchyard:carport" as carport
import "driver:fake" as fake

driverInstances = new {
  new fake.FakeInstance {
    id = "fake_one"
    enabled = false
  }
}
`)

	cp := &recordingCarport{}
	mgr, _ := NewManager(ctx, configDir, driversRoot, nopStore{}, cp)
	if err := mgr.Apply(ctx, false); err != nil {
		t.Fatal(err)
	}
	if len(cp.registered) != 0 {
		t.Fatalf("registered = %d, want 0 (disabled)", len(cp.registered))
	}
}

func TestManagerApply_MissingDriverErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	driversRoot := t.TempDir()
	configDir := t.TempDir()
	// No manifest — driver: import should fail at Pkl evaluation.
	writeMainPkl(t, configDir, `
amends "switchyard:config"
import "switchyard:carport" as carport
import "driver:fake" as fake

driverInstances = new {
  new fake.FakeInstance {
    id = "fake_one"
  }
}
`)

	cp := &recordingCarport{}
	mgr, _ := NewManager(ctx, configDir, driversRoot, nopStore{}, cp)
	if err := mgr.Apply(ctx, false); err == nil {
		t.Fatal("expected error, got nil")
	}
}
