package config

import (
	"context"
	"testing"
	"time"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
)

type fakeCarport struct {
	registered   []string
	unregistered []string
}

func (f *fakeCarport) RegisterInstance(_ context.Context, id, _ string, _ []byte) error {
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
	}

	if err := mgr.Apply(context.Background(), false); err != nil {
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
}

func TestManager_Apply_DryRun_NoSideEffects(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		DriverInstances: []*configpb.DriverInstanceConfig{
			{Id: "hue-main", DriverName: "hue", Params: []byte(`{}`)},
		},
	}
	fc := &fakeCarport{}
	fs := &fakeStore{}
	mgr := &Manager{configDir: "/fake", ev: &fakeEval{snap: snap}, store: fs, carportMgr: fc}

	if err := mgr.Apply(context.Background(), true); err != nil {
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
