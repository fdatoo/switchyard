package automation_test

import (
	"context"
	"testing"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation"
	automTestutil "github.com/fdatoo/switchyard/internal/automation/testutil"
	"github.com/fdatoo/switchyard/internal/script"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

// stateChangeTriggerAC returns a minimal AutomationConfig with a state_change
// trigger and a call_service action.
func stateChangeTriggerAC(id, entity, to string) *configpb.AutomationConfig {
	return &configpb.AutomationConfig{
		Id: id, Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{entity}, To: to},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: entity, Capability: "turn_on"},
		}}},
	}
}

func makeEngine(t *testing.T, acs ...*configpb.AutomationConfig) (*automation.Engine, *automTestutil.FakeEventStore, *automTestutil.FakeDispatcher) {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	snap := &configpb.ConfigSnapshot{Automations: acs}
	scripts, _ := script.CompileScripts(snap)
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	autos, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatal(err)
	}
	disp := &automTestutil.FakeDispatcher{}
	eng := automation.NewEngine(autos, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: disp,
		Store:      store,
		Scenes:     &automTestutil.FakeSceneApplier{},
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { eng.Stop(context.Background()) })
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return eng, store, disp
}

// waitDispatch polls until at least n calls are recorded (or times out).
func waitDispatch(t *testing.T, disp *automTestutil.FakeDispatcher, n int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(disp.GetCalls()) >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("dispatch calls: got %d, want >= %d", len(disp.GetCalls()), n)
}

// TestReload_NoChange_KeepsMatchers verifies that reloading an identical
// snapshot does NOT re-register triggers. We confirm this indirectly: the
// StateChangeMatcher tracks per-entity last state — if it were replaced the
// from-transition guard would reset, but we verify the matcher still fires
// after reload by checking dispatch is called again after the second event.
//
// More importantly: the engine should not panic and the automation should
// remain functional with unmodified trigger pointers.
func TestReload_NoChange_KeepsMatchers(t *testing.T) {
	ac := stateChangeTriggerAC("a1", "light.a", "on")
	eng, store, disp := makeEngine(t, ac)

	// Fire once.
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("light.a", true))
	waitDispatch(t, disp, 1)

	// Reload with identical config.
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{ac}}
	if err := eng.Reload(snap); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Ensure engine is still functional — fire again. ModeSingle finishes
	// before the second event; reset running flag by waiting.
	time.Sleep(50 * time.Millisecond)
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("light.a", false))
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("light.a", true))
	waitDispatch(t, disp, 2)
}

// TestReload_AddsNew verifies that a newly appearing automation is registered
// and fires on matching events.
func TestReload_AddsNew(t *testing.T) {
	ac1 := stateChangeTriggerAC("a1", "light.a", "on")
	eng, store, disp := makeEngine(t, ac1)

	// Add a second automation via reload.
	ac2 := stateChangeTriggerAC("a2", "light.b", "on")
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{ac1, ac2}}
	if err := eng.Reload(snap); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("light.b", true))
	// At least one call must be for light.b (the newly added automation's action target).
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		for _, c := range disp.GetCalls() {
			if c.Entity == "light.b" {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no dispatch for light.b after Reload added a2")
}

// TestReload_RemovesOld verifies that a removed automation no longer fires.
func TestReload_RemovesOld(t *testing.T) {
	ac1 := stateChangeTriggerAC("a1", "light.a", "on")
	ac2 := stateChangeTriggerAC("a2", "light.b", "on")
	eng, store, disp := makeEngine(t, ac1, ac2)

	// Verify a2 fires before removal.
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("light.b", true))
	waitDispatch(t, disp, 1)

	// Reload with only a1 — drop a2.
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{ac1}}
	if err := eng.Reload(snap); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Check a2 no longer in engine.
	if _, ok := eng.Get("a2"); ok {
		t.Error("a2 still present after removal")
	}

	// Send another light.b event; it must not add new calls for a2.
	callsBefore := len(disp.GetCalls())
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("light.b", false))
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("light.b", true))
	time.Sleep(100 * time.Millisecond)
	// Any new calls should not be for light.b (a2's target).
	for _, c := range disp.GetCalls()[callsBefore:] {
		if c.Entity == "light.b" {
			t.Errorf("dispatch for removed automation a2: %+v", c)
		}
	}
}

// TestReload_NoChange_MatcherPreservesLastState verifies that the
// StateChangeMatcher's last-seen state map is not reset on an identical reload.
// This is validated by using a from= constraint: if last state is preserved,
// the from check works correctly; if last was cleared, the from transition
// check could produce incorrect results.
func TestReload_NoChange_MatcherPreservesLastState(t *testing.T) {
	// Build an automation with from="off" to="on" so we can observe whether
	// the from state tracking is intact after a no-op reload.
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}

	ac := &configpb.AutomationConfig{
		Id: "from_test", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{
				Entities: []string{"light.c"}, From: "off", To: "on",
			},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: "light.c", Capability: "turn_on"},
		}}},
	}

	scripts, _ := script.CompileScripts(&configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{ac}})
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	autos, err := automation.CompileAutomations(&configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{ac}}, se, rt)
	if err != nil {
		t.Fatal(err)
	}
	disp := &automTestutil.FakeDispatcher{}
	eng := automation.NewEngine(autos, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: disp,
		Store:      store,
		Scenes:     &automTestutil.FakeSceneApplier{},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer eng.Stop(context.Background())
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Prime "last" to "off" by sending light.c=off.
	_, _ = store.Append(ctx, automTestutil.MakeLightStateEvent("light.c", false))
	time.Sleep(30 * time.Millisecond)

	// No-op reload.
	if err := eng.Reload(&configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{ac}}); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Grab the matcher — it should be the same pointer as before reload.
	// We can't do pointer equality on unexported fields, but we can observe
	// behavior: send light.c=on; it should fire (from="off" → "on" is satisfied
	// because last="off" was preserved).
	_, _ = store.Append(ctx, automTestutil.MakeLightStateEvent("light.c", true))
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(disp.GetCalls()) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("from-transition did not fire after no-op reload; last state may have been reset")
}
