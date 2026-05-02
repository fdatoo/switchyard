package automation_test

import (
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation"
	"github.com/fdatoo/switchyard/internal/script"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

func minimalAuto(id string) *configpb.AutomationConfig {
	return &configpb.AutomationConfig{
		Id: id, Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"light.a"}, To: "on"}}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: "light.b", Capability: "turn_off"}}}},
	}
}

func TestCompile_MinimalOK(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	scripts, _ := script.CompileScripts(&configpb.ConfigSnapshot{})
	se := script.NewEngine(scripts, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{minimalAuto("a1")}}
	out, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["a1"]; !ok {
		t.Fatal("missing a1")
	}
}

func TestCompile_TimeExactlyOne(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "bad", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Time{
			Time: &configpb.TimeTrigger{At: "07:30", Cron: "* * * * *"}}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: 1000}}}},
	}}}
	if _, err := automation.CompileAutomations(snap, se, rt); err == nil {
		t.Fatal("want err")
	}
}

func TestCompile_UnknownScriptRef(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "bad", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"x.y"}}}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Script{
			Script: &configpb.ScriptAction{Name: "missing"}}}},
	}}}
	if _, err := automation.CompileAutomations(snap, se, rt); err == nil {
		t.Fatal("want err")
	}
}

func TestCompile_EventTrigger_UnknownDataKey(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	se := script.NewEngine(nil, rt, script.Deps{})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "bad", Enabled: true,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_Event{
			Event: &configpb.EventTrigger{
				Kind: "driver_event",
				Data: map[string]string{"unsupported_key": "value"},
			}}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: 1000}}}},
	}}}
	if _, err := automation.CompileAutomations(snap, se, rt); err == nil {
		t.Fatal("want compile error for unsupported data key")
	}
}
