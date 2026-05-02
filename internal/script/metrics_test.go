package script_test

import (
	"context"
	"testing"

	dto "github.com/prometheus/client_model/go"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/script"
	sltestutil "github.com/fdatoo/switchyard/internal/starlark/testutil"
)

// gatherScriptMetric finds a MetricFamily by name from the registry.
func gatherScriptMetric(t *testing.T, m *observability.Metrics, name string) *dto.MetricFamily {
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

// scriptCounterValue returns the counter value matching labelPairs.
func scriptCounterValue(t *testing.T, mf *dto.MetricFamily, labelPairs map[string]string) float64 {
	t.Helper()
	if mf == nil {
		return 0
	}
	var total float64
	for _, m := range mf.GetMetric() {
		if scriptLabelsMatch(m.GetLabel(), labelPairs) {
			total += m.GetCounter().GetValue()
		}
	}
	return total
}

// scriptHistogramCount returns sample count for histograms matching labelPairs.
func scriptHistogramCount(t *testing.T, mf *dto.MetricFamily, labelPairs map[string]string) uint64 {
	t.Helper()
	if mf == nil {
		return 0
	}
	var total uint64
	for _, m := range mf.GetMetric() {
		if scriptLabelsMatch(m.GetLabel(), labelPairs) {
			total += m.GetHistogram().GetSampleCount()
		}
	}
	return total
}

func scriptLabelsMatch(pairs []*dto.LabelPair, want map[string]string) bool {
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

// TestMetrics_ScriptInvocation creates a script engine with fresh Metrics, calls
// a script, and asserts:
//
//   - switchyard_script_invocations_total{script_name, outcome="ok", invoked_by_kind="cli"} == 1
//   - switchyard_script_duration_seconds sample count > 0
//   - switchyard_script_registered gauge == 1
func TestMetrics_ScriptInvocation(t *testing.T) {
	const scriptName = "greet"

	snap := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{
		{Name: scriptName, Handler: `log("hello")`},
	}}

	scripts, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatal(err)
	}

	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	metrics := observability.NewMetrics()
	ap := &fakeAppender{}

	eng := script.NewEngine(scripts, rt, script.Deps{
		Store:   ap,
		Metrics: metrics,
	})

	_, err = eng.Call(context.Background(), scriptName, nil, "cli:testuser", "")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	// --- assert invocations_total{ok, cli} ---
	invMF := gatherScriptMetric(t, metrics, "switchyard_script_invocations_total")
	invV := scriptCounterValue(t, invMF, map[string]string{
		"script_name":     scriptName,
		"outcome":         "ok",
		"invoked_by_kind": "cli",
	})
	if invV != 1 {
		t.Errorf("script_invocations_total{ok,cli} = %.0f, want 1", invV)
	}

	// --- assert duration_seconds observed ---
	durMF := gatherScriptMetric(t, metrics, "switchyard_script_duration_seconds")
	durCount := scriptHistogramCount(t, durMF, map[string]string{"script_name": scriptName})
	if durCount == 0 {
		t.Error("script_duration_seconds: no observations")
	}

	// --- assert script_registered gauge ---
	regMF := gatherScriptMetric(t, metrics, "switchyard_script_registered")
	if regMF == nil || len(regMF.GetMetric()) == 0 {
		t.Error("script_registered: metric not found")
	} else if v := regMF.GetMetric()[0].GetGauge().GetValue(); v != 1 {
		t.Errorf("script_registered = %.0f, want 1", v)
	}
}

// TestMetrics_ScriptInvokedByKindUnknown verifies that an empty invokedBy
// maps to "unknown" for the invoked_by_kind label.
func TestMetrics_ScriptInvokedByKindUnknown(t *testing.T) {
	snap := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{
		{Name: "noop", Handler: "x = 1"},
	}}
	scripts, _ := script.CompileScripts(snap)
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	metrics := observability.NewMetrics()

	eng := script.NewEngine(scripts, rt, script.Deps{Metrics: metrics})
	_, _ = eng.Call(context.Background(), "noop", nil, "", "")

	invMF := gatherScriptMetric(t, metrics, "switchyard_script_invocations_total")
	invV := scriptCounterValue(t, invMF, map[string]string{"invoked_by_kind": "unknown"})
	if invV != 1 {
		t.Errorf("invocations_total{unknown} = %.0f, want 1", invV)
	}
}
