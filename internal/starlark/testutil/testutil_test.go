package testutil_test

import (
	"testing"

	starlarkgo "go.starlark.net/starlark"

	ghs "github.com/fdatoo/switchyard/internal/starlark"
	"github.com/fdatoo/switchyard/internal/starlark/testutil"
)

func TestNewTestRuntime_Smoke(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(
		testutil.FakeState{"light.living": {StateStr: "on", Attributes: map[string]any{}}},
		d, 42,
	)
	res := testutil.RunScript(t, rt, ghs.KindAutomation,
		`s = state("light.living")`)
	if res == nil {
		t.Fatal("expected result")
	}
}

func TestAssertCallService(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(
		testutil.FakeState{"light.kitchen": {StateStr: "off", Attributes: map[string]any{}}},
		d, 0,
	)
	testutil.RunScript(t, rt, ghs.KindAutomation,
		`call_service("light.kitchen", "turn_on")`)
	testutil.AssertCallService(t, d, "light.kitchen", "turn_on")
}

func TestAssertNoCallService(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(testutil.FakeState{}, d, 0)
	testutil.RunScript(t, rt, ghs.KindScript, `x = 1`)
	testutil.AssertNoCallService(t, d)
}

func TestAssertLog(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(testutil.FakeState{}, d, 0)
	res := testutil.RunScript(t, rt, ghs.KindScript, `log("hello")`)
	testutil.AssertLog(t, res, "hello")
}

func TestAssertValue(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(testutil.FakeState{}, d, 0)
	res := testutil.RunScript(t, rt, ghs.KindComputedEntity, `1 + 1`)
	testutil.AssertValue(t, res, starlarkgo.MakeInt(2))
}
