package starlark_test

import (
	"strings"
	"testing"

	ghs "github.com/fdatoo/switchyard/internal/starlark"
)

func TestLimitError_StepsMessage(t *testing.T) {
	err := &ghs.LimitError{Kind: ghs.LimitSteps, Context: ghs.KindAutomation, Detail: "10M steps"}
	if !strings.Contains(err.Error(), "step") {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}

func TestLimitError_WallClockMessage(t *testing.T) {
	err := &ghs.LimitError{Kind: ghs.LimitWallClock, Context: ghs.KindScript, Detail: "30s"}
	if !strings.Contains(err.Error(), "wall") {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}

func TestContextKind_String(t *testing.T) {
	cases := []struct {
		kind ghs.ContextKind
		want string
	}{
		{ghs.KindAutomation, "automation"},
		{ghs.KindComputedEntity, "computed_entity"},
		{ghs.KindTriggerCondition, "trigger_condition"},
		{ghs.KindScript, "script"},
		{ghs.KindWidgetCompute, "widget_compute"},
		{ghs.KindMCPEval, "mcp_eval"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("ContextKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestKindFromString_RoundTrip(t *testing.T) {
	for _, s := range []string{"automation", "script", "computed_entity", "trigger_condition", "widget_compute", "mcp_eval"} {
		k, err := ghs.KindFromString(s)
		if err != nil {
			t.Fatalf("KindFromString(%q): %v", s, err)
		}
		if got := k.String(); got != s {
			t.Errorf("round-trip %q → %q", s, got)
		}
	}
}

func TestKindFromString_Unknown(t *testing.T) {
	if _, err := ghs.KindFromString("bogus"); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
