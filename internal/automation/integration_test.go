//go:build integration

package automation_test

import (
	"context"
	"strings"
	"testing"
	"time"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/automation"
	"github.com/fdatoo/gohome/internal/automation/action"
	automTestutil "github.com/fdatoo/gohome/internal/automation/testutil"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/script"
	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
	sltestutil "github.com/fdatoo/gohome/internal/starlark/testutil"
)

// TestIntegration_GoldenPath uses the in-process FakeEventStore to avoid
// a real sqlite dependency. A richer store-backed scenario belongs to a
// follow-up task once we know the real store open helper.
func TestIntegration_GoldenPath(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}

	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "motion_light", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"motion.hall"}, To: "on"}}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: "light.hall", Capability: "turn_on"}}}},
	}}}

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
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer eng.Stop(context.Background())

	_, _ = store.Append(ctx, automTestutil.MakeLightStateEvent("motion.hall", true))

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(disp.GetCalls()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	calls := disp.GetCalls()
	if len(calls) == 0 {
		t.Fatal("dispatch not called")
	}
	if calls[0].Entity != "light.hall" || calls[0].Capability != "turn_on" {
		t.Fatalf("bad call %+v", calls[0])
	}
}

// buildIntegrationEng is a helper to build a fresh engine with an optional
// store/dispatcher/scenes and start it. The caller is responsible for calling
// eng.Stop.
func buildIntegrationEng(t *testing.T, snap *configpb.ConfigSnapshot) (*automation.Engine, *automTestutil.FakeEventStore, *automTestutil.FakeDispatcher, *automTestutil.FakeSceneApplier) {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	scripts, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatalf("CompileScripts: %v", err)
	}
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	autos, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatalf("CompileAutomations: %v", err)
	}
	disp := &automTestutil.FakeDispatcher{}
	scenes := &automTestutil.FakeSceneApplier{}
	eng := automation.NewEngine(autos, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: disp,
		Store:      store,
		Scenes:     scenes,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { eng.Stop(context.Background()) })
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("eng.Start: %v", err)
	}
	return eng, store, disp, scenes
}

// pollFinished polls the event store until at least one automation_finished
// event appears for the given automationID, or until the deadline.
func pollFinished(store *automTestutil.FakeEventStore, autoID string, deadline time.Time) *eventv1.AutomationFinished {
	for time.Now().Before(deadline) {
		for _, ev := range store.CopyEvents() {
			if ev.Kind != "automation_finished" {
				continue
			}
			fin := ev.Payload.GetAutomationFinished()
			if fin != nil && fin.GetAutomationId() == autoID {
				return fin
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

// countFinishedOutcomes counts automation_finished events with each outcome.
func countFinishedOutcomes(evs []eventstore.Event, autoID string) map[eventv1.RunOutcome]int {
	m := map[eventv1.RunOutcome]int{}
	for _, ev := range evs {
		if ev.Kind != "automation_finished" {
			continue
		}
		fin := ev.Payload.GetAutomationFinished()
		if fin == nil || fin.GetAutomationId() != autoID {
			continue
		}
		m[fin.GetOutcome()]++
	}
	return m
}

// TestIntegration_HoldDurationFires verifies that a StateChangeTrigger with
// forDur set fires the CallService action after the hold period elapses.
// Spec §16.3 #2 (fire path).
func TestIntegration_HoldDurationFires(t *testing.T) {
	const holdNs = int64(30 * time.Millisecond)
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "hold_fire", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{
				Entities: []string{"sensor.x"},
				To:       "on",
				ForDurNs: holdNs,
			},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: "light.x", Capability: "turn_on"},
		}}},
	}}}

	_, store, disp, _ := buildIntegrationEng(t, snap)

	// Fire the trigger state change.
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("sensor.x", true))

	// Wait well past the hold duration for the dispatch to arrive.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(disp.GetCalls()) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	calls := disp.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("want exactly 1 dispatch call after hold; got %d", len(calls))
	}
	if calls[0].Entity != "light.x" || calls[0].Capability != "turn_on" {
		t.Fatalf("unexpected call: %+v", calls[0])
	}
}

// TestIntegration_HoldDurationCancels verifies that a follow-up state change
// that breaks the hold predicate within the hold window cancels the pending
// fire so no CallService is dispatched. Spec §16.3 #2 (cancel path).
func TestIntegration_HoldDurationCancels(t *testing.T) {
	const holdNs = int64(60 * time.Millisecond)
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "hold_cancel", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{
				Entities: []string{"sensor.y"},
				To:       "on",
				ForDurNs: holdNs,
			},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: "light.y", Capability: "turn_on"},
		}}},
	}}}

	_, store, disp, _ := buildIntegrationEng(t, snap)

	// Turn on (start hold timer), then turn off within the hold window.
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("sensor.y", true))
	time.Sleep(15 * time.Millisecond) // well within the 60ms hold
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("sensor.y", false))

	// Wait past the original hold deadline to confirm no fire occurred.
	time.Sleep(200 * time.Millisecond)

	calls := disp.GetCalls()
	if len(calls) != 0 {
		t.Fatalf("want 0 dispatch calls (hold cancelled); got %d: %+v", len(calls), calls)
	}
}

// TestIntegration_ModeRestartCancelsPrior verifies that firing a restart-mode
// automation twice in quick succession cancels the first run and completes the
// second. Spec §16.3 #3.
func TestIntegration_ModeRestartCancelsPrior(t *testing.T) {
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "restarter", Enabled: true, Mode: configpb.AutomationConfig_MODE_RESTART,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"sensor.z"}},
		}}},
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_Wait{Wait: &configpb.WaitAction{DurationNs: int64(500 * time.Millisecond)}}},
			{Kind: &configpb.ActionConfig_CallService{
				CallService: &configpb.CallServiceAction{Entity: "light.z", Capability: "turn_on"},
			}},
		},
	}}}

	eng, store, disp, _ := buildIntegrationEng(t, snap)

	// Fire twice 10ms apart — first fire starts a 500ms wait, second cancels it.
	ctx := context.Background()
	if err := eng.Trigger(ctx, "restarter", "test"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := eng.Trigger(ctx, "restarter", "test"); err != nil {
		t.Fatal(err)
	}

	// Poll until both the dispatch call and the OUTCOME_OK event are recorded.
	// The engine writes the outcome event after calling the dispatcher, so we
	// must wait for both; polling only on the dispatch call races with the write.
	var counts map[eventv1.RunOutcome]int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		counts = countFinishedOutcomes(store.CopyEvents(), "restarter")
		if len(disp.GetCalls()) >= 1 && counts[eventv1.RunOutcome_OUTCOME_OK] >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	calls := disp.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("want exactly 1 dispatch call (from second fire); got %d", len(calls))
	}

	// Verify there is exactly one CANCELLED outcome in the event store.
	if counts[eventv1.RunOutcome_OUTCOME_CANCELLED] < 1 {
		t.Errorf("want >=1 CANCELLED outcome; got %v", counts)
	}
	if counts[eventv1.RunOutcome_OUTCOME_OK] != 1 {
		t.Errorf("want exactly 1 OK outcome; got %v", counts)
	}
}

// TestIntegration_ConditionShortCircuit verifies that when the first condition
// fails, subsequent conditions (including Starlark) are not evaluated and the
// run finishes with OUTCOME_CONDITION_FAIL. Spec §16.3 #5.
func TestIntegration_ConditionShortCircuit(t *testing.T) {
	// The StateCondition checks for entity "x" to equal "off".
	// Since FakeState has no entry for "x", the condition returns false immediately.
	// The Starlark condition logs a distinctive string — we assert it is absent.
	const starlarkMarker = "starlark_saw_evaluation"
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "cond_short", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"sensor.cond"}},
		}}},
		Conditions: []*configpb.ConditionConfig{
			// First: entity "x" must equal "off" — will fail (entity unknown → false).
			{Kind: &configpb.ConditionConfig_State{
				State: &configpb.StateCondition{Entity: "x", Equals: "off"},
			}},
			// Second: Starlark that would log marker — must NOT run.
			{Kind: &configpb.ConditionConfig_Starlark{
				Starlark: &configpb.StarlarkCondition{Expr: `log("` + starlarkMarker + `") or True`},
			}},
		},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: "light.cond", Capability: "turn_on"},
		}}},
	}}}

	_, store, disp, _ := buildIntegrationEng(t, snap)

	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("sensor.cond", true))

	// Wait for automation_finished.
	fin := pollFinished(store, "cond_short", time.Now().Add(500*time.Millisecond))
	if fin == nil {
		t.Fatal("no automation_finished event within timeout")
	}

	if fin.GetOutcome() != eventv1.RunOutcome_OUTCOME_CONDITION_FAIL {
		t.Errorf("want OUTCOME_CONDITION_FAIL, got %v", fin.GetOutcome())
	}

	// Starlark must not have been evaluated — no dispatch either.
	if len(disp.GetCalls()) != 0 {
		t.Errorf("want 0 dispatch calls (condition failed); got %d", len(disp.GetCalls()))
	}
	for _, line := range fin.GetLogLines() {
		if strings.Contains(line, starlarkMarker) {
			t.Errorf("Starlark was evaluated despite short-circuit: found marker in log line %q", line)
		}
	}
}

// TestIntegration_SceneStub verifies that a SceneAction uses the StubSceneApplier
// to emit a scene_applied SystemEvent and produces OUTCOME_OK. Spec §16.3 #6.
func TestIntegration_SceneStub(t *testing.T) {
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "scene_auto", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"sensor.scene"}},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Scene{
			Scene: &configpb.SceneAction{Slug: "movie_night"},
		}}},
	}}}

	// Use a StubSceneApplier wired to the same store so it emits scene_applied.
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	scripts, _ := script.CompileScripts(snap)
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	autos, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatal(err)
	}
	disp := &automTestutil.FakeDispatcher{}
	stubScenes := &action.StubSceneApplier{Store: store}
	eng := automation.NewEngine(autos, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: disp,
		Store:      store,
		Scenes:     stubScenes,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { eng.Stop(context.Background()) })
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}

	_, _ = store.Append(ctx, automTestutil.MakeLightStateEvent("sensor.scene", true))

	// Wait for automation_finished.
	fin := pollFinished(store, "scene_auto", time.Now().Add(500*time.Millisecond))
	if fin == nil {
		t.Fatal("no automation_finished event within timeout")
	}
	if fin.GetOutcome() != eventv1.RunOutcome_OUTCOME_OK {
		t.Errorf("want OUTCOME_OK, got %v; error: %s", fin.GetOutcome(), fin.GetError())
	}

	// No CommandIssued events (dispatch should not have been called).
	if len(disp.GetCalls()) != 0 {
		t.Errorf("want 0 dispatch calls; got %d: %+v", len(disp.GetCalls()), disp.GetCalls())
	}

	// A scene_applied SystemEvent must have been appended.
	found := false
	for _, ev := range store.CopyEvents() {
		if ev.Kind != "scene_applied" {
			continue
		}
		sys := ev.Payload.GetSystem()
		if sys != nil && sys.GetData()["slug"] == "movie_night" {
			found = true
			break
		}
	}
	if !found {
		t.Error("no scene_applied event found in store for slug=movie_night")
	}
}

// TestIntegration_ScriptInvocationCorrelation verifies that when an automation
// calls a script, all lifecycle events share the same correlation_id and the
// script's invoked_by field is set to "automation:<id>". Spec §16.3 #7.
func TestIntegration_ScriptInvocationCorrelation(t *testing.T) {
	const autoID = "script_caller"
	snap := &configpb.ConfigSnapshot{
		Scripts: []*configpb.ScriptConfig{{
			Name:    "greet",
			Handler: `log("hi from greet")`,
		}},
		Automations: []*configpb.AutomationConfig{{
			Id: autoID, Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
				StateChange: &configpb.StateChangeTrigger{Entities: []string{"sensor.greet"}},
			}}},
			Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Script{
				Script: &configpb.ScriptAction{Name: "greet"},
			}}},
		}},
	}

	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	scripts, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatalf("CompileScripts: %v", err)
	}
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	autos, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatalf("CompileAutomations: %v", err)
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

	_, _ = store.Append(ctx, automTestutil.MakeLightStateEvent("sensor.greet", true))

	// Wait for automation_finished.
	fin := pollFinished(store, autoID, time.Now().Add(500*time.Millisecond))
	if fin == nil {
		t.Fatal("no automation_finished event within timeout")
	}
	if fin.GetOutcome() != eventv1.RunOutcome_OUTCOME_OK {
		t.Errorf("want OUTCOME_OK, got %v; error: %s", fin.GetOutcome(), fin.GetError())
	}

	corrID := fin.GetCorrelationId()
	if corrID == "" {
		t.Fatal("automation_finished correlation_id is empty")
	}

	// Collect all lifecycle events and verify they share the correlation_id.
	evs := store.CopyEvents()
	var foundTriggered, foundScriptInvoked, foundScriptFinished bool
	var scriptInvokedBy string
	for _, ev := range evs {
		switch ev.Kind {
		case "automation_triggered":
			at := ev.Payload.GetAutomationTriggered()
			if at != nil && at.GetAutomationId() == autoID {
				if at.GetCorrelationId() != corrID {
					t.Errorf("automation_triggered corr_id mismatch: got %q want %q", at.GetCorrelationId(), corrID)
				}
				foundTriggered = true
			}
		case "script_invoked":
			si := ev.Payload.GetScriptInvoked()
			if si != nil && si.GetScriptName() == "greet" {
				if si.GetCorrelationId() != corrID {
					t.Errorf("script_invoked corr_id mismatch: got %q want %q", si.GetCorrelationId(), corrID)
				}
				scriptInvokedBy = si.GetInvokedBy()
				foundScriptInvoked = true
			}
		case "script_finished":
			sf := ev.Payload.GetScriptFinished()
			if sf != nil && sf.GetScriptName() == "greet" {
				if sf.GetCorrelationId() != corrID {
					t.Errorf("script_finished corr_id mismatch: got %q want %q", sf.GetCorrelationId(), corrID)
				}
				foundScriptFinished = true
			}
		}
	}

	if !foundTriggered {
		t.Error("automation_triggered event not found")
	}
	if !foundScriptInvoked {
		t.Error("script_invoked event not found")
	}
	if !foundScriptFinished {
		t.Error("script_finished event not found")
	}
	wantInvokedBy := "automation:" + autoID
	if scriptInvokedBy != wantInvokedBy {
		t.Errorf("script invoked_by: got %q want %q", scriptInvokedBy, wantInvokedBy)
	}
}

// TestIntegration_ParallelBlockCancellation verifies that when one child in a
// ParallelBlock errors, the sibling sleep action is interrupted and the run
// completes quickly with OUTCOME_ACTION_ERROR. Spec §16.3 #8.
func TestIntegration_ParallelBlockCancellation(t *testing.T) {
	// One child fails (divide by zero), one child sleeps 500ms.
	// errgroup cancels its context when first error is returned, so the sleep
	// is interrupted and the whole block finishes quickly.
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "par_cancel", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"sensor.par"}},
		}}},
		Actions: []*configpb.ActionConfig{{
			Kind: &configpb.ActionConfig_Parallel{
				Parallel: &configpb.ParallelBlock{
					Actions: []*configpb.ActionConfig{
						// Child 1: immediately fails.
						{Kind: &configpb.ActionConfig_Starlark{
							Starlark: &configpb.StarlarkAction{Body: `fail("oops")`},
						}},
						// Child 2: sleeps for 500ms — should be cancelled.
						{Kind: &configpb.ActionConfig_Wait{
							Wait: &configpb.WaitAction{DurationNs: int64(500 * time.Millisecond)},
						}},
					},
				},
			},
		}},
	}}}

	_, store, _, _ := buildIntegrationEng(t, snap)

	start := time.Now()
	_, _ = store.Append(context.Background(), automTestutil.MakeLightStateEvent("sensor.par", true))

	fin := pollFinished(store, "par_cancel", time.Now().Add(500*time.Millisecond))
	if fin == nil {
		t.Fatal("no automation_finished event within 500ms timeout")
	}
	elapsed := time.Since(start)

	if fin.GetOutcome() != eventv1.RunOutcome_OUTCOME_ACTION_ERROR {
		t.Errorf("want OUTCOME_ACTION_ERROR, got %v; error: %s", fin.GetOutcome(), fin.GetError())
	}

	// The run must have finished well before the 500ms sleep would have completed.
	if elapsed > 400*time.Millisecond {
		t.Errorf("parallel cancellation was too slow (%v); expected finish within ~400ms", elapsed)
	}
}

// Ensure the new imports are used; ghstarlark.EntityState is referenced through
// FakeState internally but we need ghstarlark referenced in this file to satisfy
// the compiler for the starlark runtime helpers.
var _ = (*ghstarlark.Runtime)(nil)
