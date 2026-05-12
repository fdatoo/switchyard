//go:build integration

package config

import (
	"context"
	"path/filepath"
	"testing"
)

func newDiscoveryEvaluator(t *testing.T) *pklEvaluator {
	t.Helper()
	ev, err := newPklEvaluator(context.Background(), "")
	if err != nil {
		t.Fatalf("evaluator: %v", err)
	}
	t.Cleanup(func() { _ = ev.ev.Close() })
	return ev
}

func TestDiscoverConfigDir_HappyPath(t *testing.T) {
	ev := newDiscoveryEvaluator(t)
	got, errs := discoverConfigDir(context.Background(), ev, "testdata/discovery/happy")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if len(got.Automations) != 2 {
		t.Errorf("automations: want 2, got %d", len(got.Automations))
	}
	if len(got.Areas) != 1 {
		t.Errorf("areas: want 1, got %d", len(got.Areas))
	}
	if len(got.Scenes) != 1 {
		t.Errorf("scenes: want 1, got %d", len(got.Scenes))
	}
	if len(got.EntityAreas) != 2 {
		t.Errorf("entityAreas: want 2, got %d", len(got.EntityAreas))
	}
}

func TestDiscoverConfigDir_MissingDirsAreFine(t *testing.T) {
	ev := newDiscoveryEvaluator(t)
	got, errs := discoverConfigDir(context.Background(), ev, "testdata/discovery/missing-dirs")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if len(got.Automations) != 1 {
		t.Errorf("automations: want 1, got %d", len(got.Automations))
	}
	if len(got.Areas) != 0 || len(got.Scenes) != 0 || len(got.EntityAreas) != 0 {
		t.Errorf("expected empty areas/scenes/entityAreas, got %+v", got)
	}
}

func TestDiscoverConfigDir_EmptyDirsAreFine(t *testing.T) {
	ev := newDiscoveryEvaluator(t)
	got, errs := discoverConfigDir(context.Background(), ev, "testdata/discovery/empty-dirs")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if len(got.Automations) != 0 {
		t.Errorf("want empty, got %d automations", len(got.Automations))
	}
}

func TestDiscoverConfigDir_NonPklFilesIgnored(t *testing.T) {
	ev := newDiscoveryEvaluator(t)
	got, errs := discoverConfigDir(context.Background(), ev, "testdata/discovery/non-pkl-files")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if len(got.Automations) != 1 || got.Automations[0].Path != filepath.Join("automations", "morning.pkl") {
		t.Errorf("got %+v", got.Automations)
	}
}

func TestDiscoverConfigDir_BadFileSurfacesValidationError(t *testing.T) {
	ev := newDiscoveryEvaluator(t)
	got, errs := discoverConfigDir(context.Background(), ev, "testdata/discovery/bad-file")
	if len(got.Automations) != 1 || got.Automations[0].Config.GetId() != "good" {
		t.Errorf("expected the good automation to survive, got %+v", got.Automations)
	}
	if len(errs) != 1 {
		t.Fatalf("want 1 validation error, got %d: %+v", len(errs), errs)
	}
	if errs[0].Code != "pkl_eval" {
		t.Errorf("code = %q, want pkl_eval", errs[0].Code)
	}
	if errs[0].File != filepath.Join("automations", "bad.pkl") {
		t.Errorf("file = %q", errs[0].File)
	}
}
