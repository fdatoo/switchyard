package automation_test

import (
	"context"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/automation"
	automTestutil "github.com/fdatoo/switchyard/internal/automation/testutil"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/script"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

// gatherMetric finds the MetricFamily by name from the registry.
func gatherMetric(t *testing.T, m *observability.Metrics, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

// counterValue returns the sum of all counter values matching the given label pairs.
// labelPairs is a map of label name -> value to filter on (empty map matches all).
func counterValue(t *testing.T, mf *dto.MetricFamily, labelPairs map[string]string) float64 {
	t.Helper()
	if mf == nil {
		return 0
	}
	var total float64
	for _, m := range mf.GetMetric() {
		if labelsMatch(m.GetLabel(), labelPairs) {
			total += m.GetCounter().GetValue()
		}
	}
	return total
}

// histogramCount returns the sum of all histogram sample counts matching the given labels.
func histogramCount(t *testing.T, mf *dto.MetricFamily, labelPairs map[string]string) uint64 {
	t.Helper()
	if mf == nil {
		return 0
	}
	var total uint64
	for _, m := range mf.GetMetric() {
		if labelsMatch(m.GetLabel(), labelPairs) {
			total += m.GetHistogram().GetSampleCount()
		}
	}
	return total
}

func labelsMatch(pairs []*dto.LabelPair, want map[string]string) bool {
	got := map[string]string{}
	for _, p := range pairs {
		got[p.GetName()] = p.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// TestMetrics_AutomationFire constructs an engine with fresh Metrics, fires one
// automation with a successful CallService action, then asserts:
//
//   - gohome_automation_triggers_total{automation_id, trigger_kind} == 1
//   - gohome_automation_runs_total{automation_id, outcome="ok"} == 1
//   - gohome_automation_actions_total{automation_id, action_kind="call_service", result="ok"} == 1
//   - gohome_automation_run_duration_seconds sample count > 0
//   - gohome_automation_starlark_steps sample count > 0
func TestMetrics_AutomationFire(t *testing.T) {
	const autoID = "test_auto"

	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id:      autoID,
		Enabled: true,
		Mode:    configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"motion.hall"}, To: "on"},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_CallService{
			CallService: &configpb.CallServiceAction{Entity: "light.hall", Capability: "turn_on"},
		}}},
	}}}

	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	scripts, _ := script.CompileScripts(snap)
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})

	metrics := observability.NewMetrics()

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
		Metrics:    metrics,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer eng.Stop(context.Background())

	// Fire the automation manually so we don't need to wait for state-change routing.
	if err := eng.Trigger(ctx, autoID, "cli:test"); err != nil {
		t.Fatal(err)
	}

	// Wait for the run to complete by polling for the dispatch call.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(disp.GetCalls()) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(disp.GetCalls()) == 0 {
		t.Fatal("dispatch not called; run did not complete")
	}

	// Give the goroutine time to record metrics after executeRun returns.
	time.Sleep(20 * time.Millisecond)

	// --- assert triggers_total ---
	trigMF := gatherMetric(t, metrics, "gohome_automation_triggers_total")
	trigV := counterValue(t, trigMF, map[string]string{"automation_id": autoID, "trigger_kind": "manual"})
	if trigV != 1 {
		t.Errorf("triggers_total{manual} = %.0f, want 1", trigV)
	}

	// --- assert runs_total{ok} ---
	runsMF := gatherMetric(t, metrics, "gohome_automation_runs_total")
	runsV := counterValue(t, runsMF, map[string]string{"automation_id": autoID, "outcome": "ok"})
	if runsV != 1 {
		t.Errorf("runs_total{ok} = %.0f, want 1", runsV)
	}

	// --- assert actions_total{call_service, ok} ---
	actMF := gatherMetric(t, metrics, "gohome_automation_actions_total")
	actV := counterValue(t, actMF, map[string]string{"automation_id": autoID, "action_kind": "call_service", "result": "ok"})
	if actV != 1 {
		t.Errorf("actions_total{call_service,ok} = %.0f, want 1", actV)
	}

	// --- assert run_duration_seconds observed ---
	durMF := gatherMetric(t, metrics, "gohome_automation_run_duration_seconds")
	durCount := histogramCount(t, durMF, map[string]string{"automation_id": autoID})
	if durCount == 0 {
		t.Error("run_duration_seconds: no observations")
	}

	// --- assert starlark_steps observed ---
	stepsMF := gatherMetric(t, metrics, "gohome_automation_starlark_steps")
	stepsCount := histogramCount(t, stepsMF, map[string]string{"automation_id": autoID})
	if stepsCount == 0 {
		t.Error("starlark_steps: no observations")
	}

	// --- assert automation_registered gauge ---
	regMF := gatherMetric(t, metrics, "gohome_automation_registered")
	if regMF == nil || len(regMF.GetMetric()) == 0 {
		t.Error("automation_registered: metric not found")
	} else if v := regMF.GetMetric()[0].GetGauge().GetValue(); v != 1 {
		t.Errorf("automation_registered = %.0f, want 1", v)
	}
}

// TestMetrics_AutomationSkipped checks that emitSkipped records
// gohome_automation_runs_total{outcome="skipped"}.
func TestMetrics_AutomationSkipped(t *testing.T) {
	const autoID = "single_auto"

	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id:      autoID,
		Enabled: true,
		Mode:    configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"x.y"}, To: "on"},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: int64(200 * time.Millisecond)},
		}}},
	}}}

	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	scripts, _ := script.CompileScripts(snap)
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	metrics := observability.NewMetrics()

	autos, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatal(err)
	}

	eng := automation.NewEngine(autos, se, rt, automation.Deps{
		State:   automTestutil.NewFakeState(),
		Store:   store,
		Scenes:  &automTestutil.FakeSceneApplier{},
		Metrics: metrics,
	})
	ctx, cancel := context.WithCancel(context.Background())
	if err := eng.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}

	// Fire once — this will run and block for 200ms (WaitAction).
	if err := eng.Trigger(ctx, autoID, "cli:t"); err != nil {
		cancel()
		t.Fatal(err)
	}
	// Fire again immediately — should be skipped (already running).
	time.Sleep(5 * time.Millisecond)
	if err := eng.Trigger(ctx, autoID, "cli:t"); err != nil {
		cancel()
		t.Fatal(err)
	}

	// Wait for skip metric to be recorded.
	time.Sleep(50 * time.Millisecond)

	// Stop engine and cancel context in the right order.
	cancel()
	eng.Stop(context.Background())

	runsMF := gatherMetric(t, metrics, "gohome_automation_runs_total")
	skippedV := counterValue(t, runsMF, map[string]string{"automation_id": autoID, "outcome": "skipped"})
	if skippedV < 1 {
		t.Errorf("runs_total{skipped} = %.0f, want >= 1", skippedV)
	}
}
