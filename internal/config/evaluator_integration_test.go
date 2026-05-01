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
)

func testdataDir(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestEvaluator_ValidConfig(t *testing.T) {
	ctx := context.Background()
	ev, err := newPklEvaluator(ctx)
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}

	snap, err := ev.Evaluate(ctx, testdataDir(t, "valid"))
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
	ev, err := newPklEvaluator(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snap, err := ev.Evaluate(context.Background(), dir)
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
	ev, err := newPklEvaluator(ctx)
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}
	snap, err := ev.Evaluate(ctx, testdataDir(t, "listener-defaults"))
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
	ev, err := newPklEvaluator(ctx)
	if err != nil {
		t.Fatalf("newPklEvaluator: %v", err)
	}

	snap, err := ev.Evaluate(ctx, testdataDir(t, "invalid-xref"))
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
