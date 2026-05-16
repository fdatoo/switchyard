package config

import (
	"context"
	"strings"
	"testing"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

type fakeCarport struct {
	registered   []string
	unregistered []string
}

func (f *fakeCarport) RegisterInstance(_ context.Context, id, _, _ string, _ []byte, _ bool, _ carport.LifecycleConfig) error {
	f.registered = append(f.registered, id)
	return nil
}

func (f *fakeCarport) UnregisterInstance(_ context.Context, id string) error {
	f.unregistered = append(f.unregistered, id)
	return nil
}

type fakeStore struct {
	appended []eventstore.Event
}

func (f *fakeStore) Append(_ context.Context, e eventstore.Event) (uint64, error) {
	f.appended = append(f.appended, e)
	return uint64(len(f.appended)), nil
}

// testRegistryWith returns a registry pre-populated with one driver entry per
// name. Useful for unit tests that drive Manager.Apply without going through
// the disk-scanning NewDriverRegistry path.
func testRegistryWith(names ...string) *DriverRegistry {
	r := &DriverRegistry{entries: map[string]DriverEntry{}}
	for _, n := range names {
		r.entries[n] = DriverEntry{Name: n, Version: "0.0.0", BinaryPath: "/test/bin/" + n}
	}
	return r
}

func TestManager_Apply_CallsCarportAndAppends(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		EvaluatedAtUnixMs: time.Now().UnixMilli(),
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", DriverName: "hue", Params: []byte(`{"id":"hue-main"}`)},
		},
	}

	fakeEv := &fakeEval{snap: snap}
	fc := &fakeCarport{}
	fs := &fakeStore{}

	mgr := &Manager{
		configDir:  "/fake",
		ev:         fakeEv,
		store:      fs,
		carportMgr: fc,
		registry:   testRegistryWith("hue"),
	}

	if err := mgr.Apply(context.Background(), false, "config(driver): add hue driver"); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(fc.registered) != 1 || fc.registered[0] != "hue-main" {
		t.Errorf("expected hue-main registered, got %v", fc.registered)
	}
	if len(fs.appended) != 1 {
		t.Fatalf("expected 1 event appended, got %d", len(fs.appended))
	}
	ev := fs.appended[0]
	applied := ev.Payload.GetConfigApplied()
	if applied == nil {
		t.Fatal("expected ConfigApplied payload")
	}
	if applied.DriverInstancesAdded != 1 {
		t.Errorf("expected 1 driver added, got %d", applied.DriverInstancesAdded)
	}
	if applied.Message != "config(driver): add hue driver" {
		t.Errorf("message = %q", applied.Message)
	}
}

func TestManager_Apply_DryRun_NoSideEffects(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", DriverName: "hue", Params: []byte(`{}`)},
		},
	}
	fc := &fakeCarport{}
	fs := &fakeStore{}
	mgr := &Manager{configDir: "/fake", ev: &fakeEval{snap: snap}, store: fs, carportMgr: fc, registry: testRegistryWith("hue")}

	if err := mgr.Apply(context.Background(), true, DefaultApplyMessage); err != nil {
		t.Fatalf("dry-run Apply: %v", err)
	}
	if len(fc.registered) != 0 {
		t.Errorf("dry-run should not register instances")
	}
	if len(fs.appended) != 0 {
		t.Errorf("dry-run should not append events")
	}
}

func TestManager_Current_NilBeforeApply(t *testing.T) {
	mgr := &Manager{}
	if mgr.Current() != nil {
		t.Error("Current() should be nil before Apply")
	}
}

func TestManager_Apply_StoresResolvedAndRedactedSnapshots(t *testing.T) {
	t.Setenv("TEST_API_KEY", "secret-value")
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", DriverName: "hue", Params: []byte(`{"id":"hue-main","apiKey":"env:TEST_API_KEY"}`)},
		},
	}
	mgr := &Manager{
		configDir:  "/fake",
		ev:         &fakeEval{snap: snap},
		store:      &fakeStore{},
		carportMgr: &fakeCarport{},
		registry:   testRegistryWith("hue"),
	}

	if err := mgr.Apply(context.Background(), false, DefaultApplyMessage); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := string(mgr.Current().DriverInstances[0].Params); !strings.Contains(got, "secret-value") {
		t.Fatalf("Current params not resolved: %s", got)
	}
	redacted := string(mgr.CurrentRedacted().DriverInstances[0].Params)
	if strings.Contains(redacted, "secret-value") || strings.Contains(redacted, "env:TEST_API_KEY") {
		t.Fatalf("CurrentRedacted leaked secret material: %s", redacted)
	}
	if !strings.Contains(redacted, RedactedSecret) {
		t.Fatalf("CurrentRedacted missing marker: %s", redacted)
	}
}
