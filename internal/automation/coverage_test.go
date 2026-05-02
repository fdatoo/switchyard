package automation_test

import (
	"strings"
	"testing"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation"
	automTestutil "github.com/fdatoo/switchyard/internal/automation/testutil"
	"github.com/fdatoo/switchyard/internal/script"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

// --- Mode.String coverage ---

func TestMode_StringAllVariants(t *testing.T) {
	cases := []struct {
		m    automation.Mode
		want string
	}{
		{automation.ModeSingle, "single"},
		{automation.ModeQueued, "queued"},
		{automation.ModeRestart, "restart"},
		{automation.ModeParallel, "parallel"},
		{automation.Mode(99), "single"}, // default branch
	}
	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tc.m, got, tc.want)
		}
	}
}

// --- ItemError / CompileError ---

func TestCompileError_SingleItem(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		// Missing id triggers an unset-id error.
		Enabled: true,
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: 1}}}},
	}}}
	_, err := automation.CompileAutomations(snap, se, rt)
	if err == nil {
		t.Fatal("want compile error for unset id")
	}
	// Single-item form: ItemError.Error() formats as "automations[<unset>].id: empty".
	if !strings.Contains(err.Error(), "<unset>") {
		t.Errorf("err = %q, want to mention '<unset>'", err.Error())
	}
}

func TestCompileError_MultipleItems(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	// Two automations: one duplicate-id error and one bad-trigger error → 2 items.
	bad := &configpb.AutomationConfig{
		Id: "a1", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{}, // entities empty → error
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: 1}}}},
	}
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{bad,
		{Id: "", Enabled: true}, // unset id → second item error
	}}
	_, err := automation.CompileAutomations(snap, se, rt)
	if err == nil {
		t.Fatal("want compile error")
	}
	if !strings.Contains(err.Error(), "compile errors") {
		t.Errorf("err = %q, want plural-form message", err.Error())
	}
}

// --- compileTrigger error paths ---

func TestCompile_StateChangeRequiresEntities(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "bad", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: 1}}}},
	}}}
	if _, err := automation.CompileAutomations(snap, se, rt); err == nil {
		t.Fatal("want err: state_change requires entities")
	}
}

func TestCompile_EventTriggerRequiresKind(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "bad", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Event{
			Event: &configpb.EventTrigger{}, // no kind
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: 1}}}},
	}}}
	if _, err := automation.CompileAutomations(snap, se, rt); err == nil {
		t.Fatal("want err: event trigger requires kind")
	}
}

// --- compileCondition: every variant + error paths ---

func compileWithCondition(t *testing.T, c *configpb.ConditionConfig) error {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "a1", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"x.y"}, To: "on"}}}},
		Conditions: []*configpb.ConditionConfig{c},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: 1}}}},
	}}}
	_, err := automation.CompileAutomations(snap, se, rt)
	return err
}

func TestCompile_ConditionState_ExactlyOneOperator(t *testing.T) {
	// No operator
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_State{
		State: &configpb.StateCondition{Entity: "x.y"}}}); err == nil {
		t.Fatal("want err: no operator")
	}
	// Two operators
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_State{
		State: &configpb.StateCondition{Entity: "x.y", Equals: "on", Not: "off"}}}); err == nil {
		t.Fatal("want err: two operators")
	}
	// OneOf alone is fine
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_State{
		State: &configpb.StateCondition{Entity: "x.y", OneOf: []string{"a", "b"}}}}); err != nil {
		t.Fatalf("OneOf should compile: %v", err)
	}
}

func TestCompile_ConditionNumeric_BadOp(t *testing.T) {
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Numeric{
		Numeric: &configpb.NumericCondition{Entity: "x.y", Op: "wat", Value: 1}}}); err == nil {
		t.Fatal("want err: bad op")
	}
}

func TestCompile_ConditionTime_OK(t *testing.T) {
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Time{
		Time: &configpb.TimeCondition{After: "08:00", Before: "20:00"}}}); err != nil {
		t.Fatalf("time should compile: %v", err)
	}
}

func TestCompile_ConditionStarlark(t *testing.T) {
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Starlark{
		Starlark: &configpb.StarlarkCondition{Expr: "1 + 1 == 2"}}}); err != nil {
		t.Fatalf("starlark should compile: %v", err)
	}
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Starlark{
		Starlark: &configpb.StarlarkCondition{Expr: "def broken("}}}); err == nil {
		t.Fatal("want err: bad starlark")
	}
}

func TestCompile_ConditionAnd(t *testing.T) {
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_And{
		And: &configpb.AndCondition{}}}); err == nil {
		t.Fatal("want err: empty and")
	}
	good := &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_And{
		And: &configpb.AndCondition{All: []*configpb.ConditionConfig{
			{Kind: &configpb.ConditionConfig_State{State: &configpb.StateCondition{Entity: "x.y", Equals: "on"}}},
		}}}}
	if err := compileWithCondition(t, good); err != nil {
		t.Fatalf("nonempty And should compile: %v", err)
	}
}

func TestCompile_ConditionOr(t *testing.T) {
	if err := compileWithCondition(t, &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Or{
		Or: &configpb.OrCondition{}}}); err == nil {
		t.Fatal("want err: empty or")
	}
	good := &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Or{
		Or: &configpb.OrCondition{Any: []*configpb.ConditionConfig{
			{Kind: &configpb.ConditionConfig_State{State: &configpb.StateCondition{Entity: "x.y", Equals: "on"}}},
		}}}}
	if err := compileWithCondition(t, good); err != nil {
		t.Fatalf("nonempty Or should compile: %v", err)
	}
}

func TestCompile_ConditionNot(t *testing.T) {
	good := &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Not{
		Not: &configpb.NotCondition{Not: &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_State{
			State: &configpb.StateCondition{Entity: "x.y", Equals: "on"}}}}}}
	if err := compileWithCondition(t, good); err != nil {
		t.Fatalf("Not should compile: %v", err)
	}
	// Not of bad inner propagates
	bad := &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Not{
		Not: &configpb.NotCondition{Not: &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_State{
			State: &configpb.StateCondition{Entity: "x.y"}}}}}} // no operator
	if err := compileWithCondition(t, bad); err == nil {
		t.Fatal("want err: Not of invalid inner")
	}
}

// --- compileAction: every variant + error paths ---

func compileWithAction(t *testing.T, a *configpb.ActionConfig) error {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	scripts, _ := script.CompileScripts(&configpb.ConfigSnapshot{})
	se := script.NewEngine(scripts, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "a1", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"x.y"}, To: "on"}}}},
		Actions: []*configpb.ActionConfig{a},
	}}}
	_, err := automation.CompileAutomations(snap, se, rt)
	return err
}

func TestCompile_ActionCallService_RequiresEntityCapability(t *testing.T) {
	if err := compileWithAction(t, &configpb.ActionConfig{Kind: &configpb.ActionConfig_CallService{
		CallService: &configpb.CallServiceAction{}}}); err == nil {
		t.Fatal("want err: call_service missing fields")
	}
	if err := compileWithAction(t, &configpb.ActionConfig{Kind: &configpb.ActionConfig_CallService{
		CallService: &configpb.CallServiceAction{Entity: "light.x", Capability: "turn_on"}}}); err != nil {
		t.Fatalf("valid call_service should compile: %v", err)
	}
}

func TestCompile_ActionScene(t *testing.T) {
	if err := compileWithAction(t, &configpb.ActionConfig{Kind: &configpb.ActionConfig_Scene{
		Scene: &configpb.SceneAction{Slug: "evening"}}}); err != nil {
		t.Fatalf("scene should compile: %v", err)
	}
}

func TestCompile_ActionScript_UnknownName(t *testing.T) {
	if err := compileWithAction(t, &configpb.ActionConfig{Kind: &configpb.ActionConfig_Script{
		Script: &configpb.ScriptAction{Name: "missing_script"}}}); err == nil {
		t.Fatal("want err: unknown script ref")
	}
}

func TestCompile_ActionStarlark(t *testing.T) {
	if err := compileWithAction(t, &configpb.ActionConfig{Kind: &configpb.ActionConfig_Starlark{
		Starlark: &configpb.StarlarkAction{Body: "x = 1"}}}); err != nil {
		t.Fatalf("good starlark should compile: %v", err)
	}
	if err := compileWithAction(t, &configpb.ActionConfig{Kind: &configpb.ActionConfig_Starlark{
		Starlark: &configpb.StarlarkAction{Body: "def broken("}}}); err == nil {
		t.Fatal("want err: bad starlark body")
	}
}

func TestCompile_ActionSequenceAndParallel(t *testing.T) {
	leaf := &configpb.ActionConfig{Kind: &configpb.ActionConfig_Wait{
		Wait: &configpb.WaitAction{DurationNs: 1}}}
	seq := &configpb.ActionConfig{Kind: &configpb.ActionConfig_Sequence{
		Sequence: &configpb.SequenceBlock{Actions: []*configpb.ActionConfig{leaf, leaf}}}}
	if err := compileWithAction(t, seq); err != nil {
		t.Fatalf("sequence should compile: %v", err)
	}
	par := &configpb.ActionConfig{Kind: &configpb.ActionConfig_Parallel{
		Parallel: &configpb.ParallelBlock{Actions: []*configpb.ActionConfig{leaf, leaf}}}}
	if err := compileWithAction(t, par); err != nil {
		t.Fatalf("parallel should compile: %v", err)
	}
}

func TestCompile_ActionSequence_PropagatesChildErr(t *testing.T) {
	bad := &configpb.ActionConfig{Kind: &configpb.ActionConfig_CallService{
		CallService: &configpb.CallServiceAction{}}} // missing entity
	seq := &configpb.ActionConfig{Kind: &configpb.ActionConfig_Sequence{
		Sequence: &configpb.SequenceBlock{Actions: []*configpb.ActionConfig{bad}}}}
	if err := compileWithAction(t, seq); err == nil {
		t.Fatal("want err: sequence with bad child")
	}
}

func TestCompile_ActionParallel_PropagatesChildErr(t *testing.T) {
	bad := &configpb.ActionConfig{Kind: &configpb.ActionConfig_CallService{
		CallService: &configpb.CallServiceAction{}}}
	par := &configpb.ActionConfig{Kind: &configpb.ActionConfig_Parallel{
		Parallel: &configpb.ParallelBlock{Actions: []*configpb.ActionConfig{bad}}}}
	if err := compileWithAction(t, par); err == nil {
		t.Fatal("want err: parallel with bad child")
	}
}

// --- Engine.SetEnabled error path ---

func TestEngine_SetEnabledUnknownIDErrors(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	eng := automation.NewEngine(map[string]*automation.Automation{}, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: &automTestutil.FakeDispatcher{},
		Store:      &automTestutil.FakeEventStore{},
		Scenes:     &automTestutil.FakeSceneApplier{},
	})
	if err := eng.SetEnabled("ghost", true); err == nil {
		t.Fatal("want err: unknown automation id")
	}
}

func TestEngine_GetUnknownReturnsFalse(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	eng := automation.NewEngine(map[string]*automation.Automation{}, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: &automTestutil.FakeDispatcher{},
		Store:      &automTestutil.FakeEventStore{},
		Scenes:     &automTestutil.FakeSceneApplier{},
	})
	if _, ok := eng.Get("ghost"); ok {
		t.Fatal("Get(ghost) should report not found")
	}
}

// --- Engine constructed with all 4 trigger kinds (registerTriggers all branches) ---

func TestEngine_RegisterAllTriggerKinds(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	scripts, _ := script.CompileScripts(&configpb.ConfigSnapshot{})
	se := script.NewEngine(scripts, rt, script.Deps{})
	wait := &configpb.ActionConfig{Kind: &configpb.ActionConfig_Wait{
		Wait: &configpb.WaitAction{DurationNs: 1}}}
	autos := []*configpb.AutomationConfig{
		{Id: "sc", Enabled: true,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
				StateChange: &configpb.StateChangeTrigger{Entities: []string{"x.y"}, To: "on"}}}},
			Actions: []*configpb.ActionConfig{wait}},
		{Id: "ev", Enabled: true,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Event{
				Event: &configpb.EventTrigger{Kind: "ping"}}}},
			Actions: []*configpb.ActionConfig{wait}},
		{Id: "wh", Enabled: true,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Webhook{
				Webhook: &configpb.WebhookTrigger{Path: "doorbell"}}}},
			Actions: []*configpb.ActionConfig{wait}},
		{Id: "tAt", Enabled: true,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Time{
				Time: &configpb.TimeTrigger{At: "08:00"}}}},
			Actions: []*configpb.ActionConfig{wait}},
		{Id: "tCron", Enabled: true,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Time{
				Time: &configpb.TimeTrigger{Cron: "*/5 * * * *"}}}},
			Actions: []*configpb.ActionConfig{wait}},
		{Id: "tEvery", Enabled: true,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Time{
				Time: &configpb.TimeTrigger{EveryNs: int64(time.Hour)}}}},
			Actions: []*configpb.ActionConfig{wait}},
	}
	snap := &configpb.ConfigSnapshot{Automations: autos}
	out, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	eng := automation.NewEngine(out, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: &automTestutil.FakeDispatcher{},
		Store:      &automTestutil.FakeEventStore{},
		Scenes:     &automTestutil.FakeSceneApplier{},
	})
	if got := len(eng.List()); got != len(autos) {
		t.Errorf("List len = %d, want %d", got, len(autos))
	}
}
