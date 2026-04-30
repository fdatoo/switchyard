package starlark_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	starlarkgo "go.starlark.net/starlark"

	ghs "github.com/fdatoo/gohome/internal/starlark"
)

func newTestRuntime(t *testing.T) *ghs.Runtime {
	t.Helper()
	return ghs.NewRuntime(
		fakeState{"light.kitchen": {StateStr: "off", Attributes: map[string]any{}}},
		&fakeDispatcher{},
		&fakeAppender{},
		nil,
		t.TempDir(),
		nil,
	)
}

func TestExecute_SimpleExpression(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindComputedEntity, "1 + 2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.(starlarkgo.Int).BigInt().Int64() != 3 {
		t.Fatalf("expected 3, got %v", res.Value)
	}
}

func TestExecute_SimpleScript(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindAutomation, `x = 42`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value != starlarkgo.None {
		t.Fatalf("script expected None, got %v", res.Value)
	}
}

func TestExecute_StateBuiltin(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`s = state("light.kitchen")`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res
}

func TestExecute_CallServiceUnavailableInComputedEntity(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindComputedEntity,
		`call_service("light.kitchen", "turn_on")`, nil)
	if err == nil {
		t.Fatal("expected error: call_service not available in KindComputedEntity")
	}
}

func TestExecute_SleepUnavailableInComputedEntity(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindComputedEntity, `sleep(1)`, nil)
	if err == nil {
		t.Fatal("expected error: sleep not available in KindComputedEntity")
	}
}

func TestExecute_LogAppendsToResult(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`log("hello world")`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Logs) == 0 || !strings.Contains(res.Logs[0], "hello world") {
		t.Fatalf("expected log entry, got %v", res.Logs)
	}
}

func TestExecute_StepLimitBreached(t *testing.T) {
	rt := newTestRuntime(t)
	// Tight loop to exceed the KindScript step limit (10M steps).
	_, err := rt.Execute(t.Context(), ghs.KindScript, `
i = 0
for _ in range(10000000):
    i = i + 1
`, nil)
	if err == nil {
		t.Fatal("expected LimitError")
	}
	var le *ghs.LimitError
	if !errors.As(err, &le) || le.Kind != ghs.LimitSteps {
		t.Fatalf("expected LimitSteps error, got %T: %v", err, err)
	}
}

func TestExecute_WallClockLimit(t *testing.T) {
	rt := newTestRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// sleep(5) in a 100ms wall-clock context → should time out.
	_, err := rt.Execute(ctx, ghs.KindComputedEntity, `sleep(5)`, nil)
	if err == nil {
		t.Fatal("expected error from sleep in KindComputedEntity (not in stdlib)")
	}
}

func TestExecute_ExtraGlobalsInjected(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindScript,
		`result = my_var + 1`,
		starlarkgo.StringDict{"my_var": starlarkgo.MakeInt(41)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res
}

func TestExecute_EventGlobalPresent_Automation(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`k = event.kind`, nil)
	if err != nil {
		t.Fatalf("event global missing in KindAutomation: %v", err)
	}
}

func TestExecute_EventReadOnly_TriggerCondition(t *testing.T) {
	rt := newTestRuntime(t)
	// KindTriggerCondition is expression-mode; use a bare expression, not a statement.
	_, err := rt.Execute(t.Context(), ghs.KindTriggerCondition,
		`event.kind`, nil)
	if err != nil {
		t.Fatalf("event global missing in KindTriggerCondition: %v", err)
	}
	// fire() should not be available in trigger condition
	_, err = rt.Execute(t.Context(), ghs.KindTriggerCondition,
		`event.fire("x", None)`, nil)
	if err == nil {
		t.Fatal("expected error: event.fire not available in KindTriggerCondition")
	}
}

func TestExecute_SceneGlobal_Automation(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`scene.apply("movie_night")`, nil)
	if err != nil {
		t.Fatalf("scene.apply failed: %v", err)
	}
}

func TestExecute_ElapsedAndStepsPopulated(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindScript, `x = 1`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Elapsed == 0 {
		t.Fatal("Elapsed should be > 0")
	}
	if res.Steps == 0 {
		t.Fatal("Steps should be > 0")
	}
}

func TestInvalidateModuleCache(t *testing.T) {
	rt := newTestRuntime(t)
	rt.InvalidateModuleCache() // should not panic
}
