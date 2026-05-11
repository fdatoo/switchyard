package regen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation/regen"
)

// helper: load a golden file, or return "" if it doesn't exist yet.
func loadGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name+".pkl")
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// helper: write a golden file.
func writeGolden(t *testing.T, name string, content []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".pkl")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writeGolden %s: %v", name, err)
	}
}

func runGolden(t *testing.T, name string, ac *configpb.AutomationConfig) {
	t.Helper()
	out, err := regen.Render(ac)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	golden := loadGolden(t, name)
	if golden == "" {
		writeGolden(t, name, out)
		t.Logf("wrote golden %s.pkl", name)
		return
	}
	if string(out) != golden {
		t.Errorf("output mismatch for %s:\n=== got ===\n%s\n=== want ===\n%s", name, out, golden)
	}
}

func TestRender_TimeTrigger(t *testing.T) {
	ac := &configpb.AutomationConfig{
		Id:      "night-mode",
		Enabled: true,
		Triggers: []*configpb.TriggerConfig{
			{Kind: &configpb.TriggerConfig_Time{Time: &configpb.TimeTrigger{At: "21:30"}}},
		},
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity:     "light.living_room",
				Capability: "turn_off",
			}}},
		},
	}
	out, err := regen.Render(ac)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `at = "21:30"`) {
		t.Errorf("missing at field in output: %s", s)
	}
	if !strings.Contains(s, "TimeTrigger") {
		t.Errorf("missing TimeTrigger in output: %s", s)
	}
	runGolden(t, "time_trigger", ac)
}

func TestRender_StateChangeTrigger(t *testing.T) {
	ac := &configpb.AutomationConfig{
		Id:      "motion-lights",
		Enabled: true,
		Triggers: []*configpb.TriggerConfig{
			{Kind: &configpb.TriggerConfig_StateChange{StateChange: &configpb.StateChangeTrigger{
				Entities: []string{"binary_sensor.motion"},
				To:       "on",
			}}},
		},
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity:     "light.hallway",
				Capability: "turn_on",
			}}},
		},
	}
	out, err := regen.Render(ac)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "StateChangeTrigger") {
		t.Errorf("missing StateChangeTrigger in output: %s", s)
	}
	if !strings.Contains(s, `to = "on"`) {
		t.Errorf("missing to field in output: %s", s)
	}
	runGolden(t, "state_change_trigger", ac)
}

func TestRender_StarlarkCondition_LockedAnnotation(t *testing.T) {
	ac := &configpb.AutomationConfig{
		Id:      "smart-lights",
		Enabled: true,
		Triggers: []*configpb.TriggerConfig{
			{Kind: &configpb.TriggerConfig_Time{Time: &configpb.TimeTrigger{At: "20:00"}}},
		},
		Conditions: []*configpb.ConditionConfig{
			{Kind: &configpb.ConditionConfig_Starlark{Starlark: &configpb.StarlarkCondition{
				Expr: `state("sensor.lux").value < 50`,
			}}},
		},
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity:     "light.garden",
				Capability: "turn_on",
			}}},
		},
	}
	out, err := regen.Render(ac)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "// locked-region:starlark") {
		t.Errorf("missing locked-region marker in output: %s", s)
	}
	if !strings.Contains(s, "// end-locked-region") {
		t.Errorf("missing end-locked-region marker in output: %s", s)
	}
	runGolden(t, "starlark_condition_locked", ac)
}

func TestRender_OnFailure_Retry(t *testing.T) {
	ac := &configpb.AutomationConfig{
		Id:      "critical-alert",
		Enabled: true,
		Triggers: []*configpb.TriggerConfig{
			{Kind: &configpb.TriggerConfig_Event{Event: &configpb.EventTrigger{Kind: "switchyard.manual"}}},
		},
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity:     "notify.phone",
				Capability: "notify",
			}}},
		},
		OnFailure: &configpb.OnFailureConfig{
			Strategy: &configpb.OnFailureConfig_Retry{
				Retry: &configpb.RetryStrategy{
					MaxAttempts: 3,
					BackoffNs:   5_000_000_000, // 5s
				},
			},
		},
	}
	out, err := regen.Render(ac)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "RetryStrategy") {
		t.Errorf("missing RetryStrategy in output: %s", s)
	}
	if !strings.Contains(s, "maxAttempts = 3") {
		t.Errorf("missing maxAttempts in output: %s", s)
	}
	runGolden(t, "onfailure_retry", ac)
}

func TestRender_Deterministic(t *testing.T) {
	ac := &configpb.AutomationConfig{
		Id:      "determ-test",
		Enabled: true,
		Triggers: []*configpb.TriggerConfig{
			{Kind: &configpb.TriggerConfig_Time{Time: &configpb.TimeTrigger{At: "08:00"}}},
		},
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity:     "light.kitchen",
				Capability: "turn_on",
			}}},
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity:     "light.hallway",
				Capability: "turn_on",
			}}},
		},
	}
	first, err := regen.Render(ac)
	if err != nil {
		t.Fatalf("first Render: %v", err)
	}
	for i := 0; i < 19; i++ {
		out, err := regen.Render(ac)
		if err != nil {
			t.Fatalf("Render iteration %d: %v", i, err)
		}
		if !bytes.Equal(first, out) {
			t.Fatalf("non-deterministic output at iteration %d", i+1)
		}
	}
}
