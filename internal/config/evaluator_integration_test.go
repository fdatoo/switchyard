//go:build integration

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation/regen"
)

func testdataDir(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestEvaluator_ValidConfig(t *testing.T) {
	ctx := context.Background()
	ev, err := newPklEvaluator(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}

	snap, _, err := ev.Evaluate(ctx, testdataDir(t, "valid"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(snap.GetDriverInstances()) != 1 {
		t.Errorf("expected 1 driver instance, got %d", len(snap.GetDriverInstances()))
	}
	if snap.GetDriverInstances()[0].GetId() != "fake-main" {
		t.Errorf("unexpected id: %s", snap.GetDriverInstances()[0].GetId())
	}
	if len(snap.GetEntities()) != 1 {
		t.Errorf("expected 1 entity, got %d", len(snap.GetEntities()))
	}
	if snap.GetEntities()[0].GetId() != "light.living_room" {
		t.Errorf("unexpected entity id: %s", snap.GetEntities()[0].GetId())
	}
}

func writePkl(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustEvaluate(t *testing.T, dir string) *configpb.ConfigSnapshot {
	t.Helper()
	ev, err := newPklEvaluator(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	snap, _, err := ev.Evaluate(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestEvaluate_TypedAutomation(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `
amends "switchyard:config"

import "switchyard:automations" as switchyard_automations

automations = new {
  new {
    id = "evening_lights"
    triggers = new {
      new switchyard_automations.StateChangeTrigger {
        entities = new { "motion.hallway" }
        from = "off"
        to = "on"
        forDur = 5.s
      }
    }
    conditions = new {
      new switchyard_automations.StateCondition {
        entity = "light.hallway"
        equals = "off"
      }
    }
    actions = new {
      new switchyard_automations.CallServiceAction {
        entity = "light.hallway"
        capability = "turn_on"
      }
    }
    mode = "single"
  }
}
`)
	snap := mustEvaluate(t, dir)
	if got := len(snap.Automations); got != 1 {
		t.Fatalf("want 1 automation, got %d", got)
	}
	a := snap.Automations[0]
	if a.GetId() != "evening_lights" {
		t.Errorf("id = %q", a.GetId())
	}
	if len(a.GetTriggers()) != 1 {
		t.Fatalf("want 1 trigger, got %d", len(a.GetTriggers()))
	}
	sc := a.GetTriggers()[0].GetStateChange()
	if sc == nil {
		t.Fatal("want state_change trigger")
	}
	if got := sc.GetEntities(); !reflect.DeepEqual(got, []string{"motion.hallway"}) {
		t.Errorf("entities = %v", got)
	}
	if sc.GetForDurNs() != int64(5*time.Second) {
		t.Errorf("forDur = %d", sc.GetForDurNs())
	}
	if a.GetMode() != configpb.AutomationConfig_MODE_SINGLE {
		t.Errorf("mode = %v", a.GetMode())
	}
}

func TestEvaluate_ScenesDeclaredInline(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `
amends "switchyard:config"

import "switchyard:scenes" as sc
import "switchyard:automations" as auto

scenes = new {
  new sc.Scene {
    id = "movie-night"
    displayName = "Movie Night"
    actions = new {
      new auto.CallServiceAction {
        entity = "light.living_room"
        capability = "turn_off"
      }
    }
  }
}
`)
	snap := mustEvaluate(t, dir)
	if got := len(snap.GetScenes()); got != 1 {
		t.Fatalf("want 1 scene, got %d", got)
	}
	s := snap.GetScenes()[0]
	if s.GetId() != "movie-night" {
		t.Errorf("id = %q", s.GetId())
	}
	if s.GetDisplayName() != "Movie Night" {
		t.Errorf("displayName = %q", s.GetDisplayName())
	}
	if len(s.GetActions()) != 1 {
		t.Errorf("want 1 action, got %d", len(s.GetActions()))
	}
}

func TestEvaluate_BrokenDiscoveryFileIsSoft(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `amends "switchyard:config"`)
	if err := os.MkdirAll(filepath.Join(dir, "automations"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePkl(t, filepath.Join(dir, "automations"), "good.pkl", `
amends "switchyard:automation"
import "switchyard:automations" as auto

id = "good"
enabled = true
triggers {
  new auto.EventTrigger { kind = "sun.sunset" }
}
actions {}
`)
	writePkl(t, filepath.Join(dir, "automations"), "bad.pkl", `
amends "switchyard:automation"

id = unterminated_token
`)

	ev, err := newPklEvaluator(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ev.ev.Close() }()

	snap, validationErrs, err := ev.Evaluate(context.Background(), dir)
	if err != nil {
		t.Fatalf("Evaluate should not hard-fail on a single bad file: %v", err)
	}

	foundGood := false
	for _, a := range snap.GetAutomations() {
		if a.GetId() == "good" {
			foundGood = true
		}
	}
	if !foundGood {
		t.Error("expected 'good' automation to be loaded despite bad sibling")
	}

	if len(validationErrs) < 1 {
		t.Fatalf("expected at least 1 validation error for bad.pkl, got %+v", validationErrs)
	}
	badErr := validationErrs[0]
	if badErr.Code != "pkl_eval" {
		t.Errorf("err code = %q, want pkl_eval", badErr.Code)
	}
	if badErr.File != filepath.Join("automations", "bad.pkl") {
		t.Errorf("err file = %q", badErr.File)
	}
}

// TestEvaluate_LoopClosure_RegenToSnapshot proves the contract that the
// "+ New automation" UX promised: render an AutomationConfig with regen,
// write the bytes to <configDir>/automations/<id>.pkl, evaluate the
// config — and find the automation in the live snapshot. This is the
// originally-broken end-to-end path the auto-discovery feature exists
// to close.
func TestEvaluate_LoopClosure_RegenToSnapshot(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `amends "switchyard:config"`)
	if err := os.MkdirAll(filepath.Join(dir, "automations"), 0o755); err != nil {
		t.Fatal(err)
	}

	ac := &configpb.AutomationConfig{
		Id:      "loop-test",
		Enabled: true,
		Triggers: []*configpb.TriggerConfig{
			{Kind: &configpb.TriggerConfig_Event{Event: &configpb.EventTrigger{Kind: "sun.sunset"}}},
		},
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity:     "light.living_room",
				Capability: "turn_on",
			}}},
		},
	}
	pklBytes, err := regen.Render(ac)
	if err != nil {
		t.Fatalf("regen.Render: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "automations", "loop-test.pkl"), pklBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	ev, err := newPklEvaluator(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ev.ev.Close() }()

	snap, validationErrs, err := ev.Evaluate(context.Background(), dir)
	if err != nil {
		t.Fatalf("Evaluate: %v (soft errs: %+v)", err, validationErrs)
	}
	if len(validationErrs) != 0 {
		t.Errorf("unexpected validation errors: %+v", validationErrs)
	}

	found := false
	for _, a := range snap.GetAutomations() {
		if a.GetId() == "loop-test" {
			found = true
			if len(a.GetTriggers()) != 1 || a.GetTriggers()[0].GetEvent() == nil {
				t.Errorf("automation triggers not decoded: %+v", a.GetTriggers())
			}
			if len(a.GetActions()) != 1 || a.GetActions()[0].GetCallService() == nil {
				t.Errorf("automation actions not decoded: %+v", a.GetActions())
			}
		}
	}
	if !found {
		t.Fatalf("'loop-test' automation missing from snapshot — auto-discovery loop is broken")
	}
}

func TestEvaluate_DuplicateIdIsHardError(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `
amends "switchyard:config"

import "switchyard:automations" as auto

automations = new {
  new {
    id = "dup"
    enabled = true
    triggers = new {
      new auto.EventTrigger { kind = "sun.sunset" }
    }
    actions = new {}
  }
}
`)
	if err := os.MkdirAll(filepath.Join(dir, "automations"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePkl(t, filepath.Join(dir, "automations"), "dup.pkl", `
amends "switchyard:automation"
import "switchyard:automations" as auto

id = "dup"
enabled = true
triggers {
  new auto.EventTrigger { kind = "sun.sunset" }
}
actions {}
`)

	ev, err := newPklEvaluator(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ev.ev.Close() }()

	_, validationErrs, err := ev.Evaluate(context.Background(), dir)
	if err == nil {
		t.Fatal("expected hard error for duplicate id")
	}
	found := false
	for _, e := range validationErrs {
		if e.Code == "duplicate_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate_id soft error alongside hard error, got %+v", validationErrs)
	}
}

func TestEvaluate_TypedScript(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `
amends "switchyard:config"

import "switchyard:scripts" as switchyard_scripts

scripts = new {
  new {
    name = "greet"
    params = new {
      new { name = "who"; type = "string" }
    }
    handler = """
def main(params):
    log("hello " + params["who"])
"""
  }
}
`)
	snap := mustEvaluate(t, dir)
	if len(snap.Scripts) != 1 {
		t.Fatalf("want 1 script, got %d", len(snap.Scripts))
	}
	s := snap.Scripts[0]
	if s.GetName() != "greet" {
		t.Errorf("name = %q", s.GetName())
	}
	if len(s.GetParams()) != 1 {
		t.Fatalf("want 1 param, got %d", len(s.GetParams()))
	}
	if s.GetParams()[0].GetType() != configpb.ScriptParam_TYPE_STRING {
		t.Errorf("param type = %v", s.GetParams()[0].GetType())
	}
}

func TestEvaluate_ListenerDefaults(t *testing.T) {
	ctx := context.Background()
	ev, err := newPklEvaluator(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}
	snap, _, err := ev.Evaluate(ctx, testdataDir(t, "listener-defaults"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	l := snap.GetListener()
	if l == nil {
		t.Fatal("listener is nil")
	}
	if l.GetTcp().GetBind() != "127.0.0.1:8080" {
		t.Errorf("tcp bind = %q", l.GetTcp().GetBind())
	}
	if l.GetUds().GetMode() != 0o600 {
		t.Errorf("uds mode = %o", l.GetUds().GetMode())
	}
	if l.GetStreamHeartbeatIntervalMs() != 30000 {
		t.Errorf("hb = %d ms", l.GetStreamHeartbeatIntervalMs())
	}
}

func TestEvaluator_InvalidXref(t *testing.T) {
	ctx := context.Background()
	ev, err := newPklEvaluator(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}

	snap, _, err := ev.Evaluate(ctx, testdataDir(t, "invalid-xref"))
	if err != nil {
		// Pkl's type constraint caught it — acceptable.
		var ee *EvalError
		if !errors.As(err, &ee) {
			t.Fatalf("expected *EvalError, got %T: %v", err, err)
		}
		return
	}
	// Pkl accepted it (regex constraint removed?). Compile must still catch it.
	errs := Compile(snap, nil)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for invalid-xref fixture")
	}
	found := false
	for _, e := range errs {
		if e.Field != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a ValidationError with a non-empty Field, got: %v", errs)
	}
}

func TestEvaluate_SceneWithAreaId(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `
amends "switchyard:config"

import "switchyard:scenes" as sc
import "switchyard:automations" as auto

scenes = new {
  new sc.Scene {
    id = "kitchen-bright"
    displayName = "Kitchen bright"
    areaId = "kitchen"
    actions = new {
      new auto.CallServiceAction {
        entity = "light.kitchen"
        capability = "turn_on"
      }
    }
  }
}
`)
	snap := mustEvaluate(t, dir)
	if len(snap.GetScenes()) != 1 {
		t.Fatalf("want 1 scene, got %d", len(snap.GetScenes()))
	}
	if got := snap.GetScenes()[0].GetAreaId(); got != "kitchen" {
		t.Errorf("area_id = %q, want kitchen", got)
	}
}
