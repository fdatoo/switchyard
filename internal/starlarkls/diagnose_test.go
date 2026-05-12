package starlarkls

import (
	"strings"
	"testing"
)

func diag(t *testing.T, source string, symbols map[string]SymbolInfo) []Diagnostic {
	t.Helper()
	return Diagnose("test.star", source, symbols, predeclaredNames())
}

func TestDiagnose_HappyPath(t *testing.T) {
	src := `def foo():
    return 1

def bar():
    return foo()
`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}

func TestDiagnose_ParseError(t *testing.T) {
	src := `def foo(`
	got := diag(t, src, nil)
	if len(got) != 1 {
		t.Fatalf("want 1 diagnostic, got %v", got)
	}
	if got[0].Code != "parse_error" || got[0].Severity != "error" {
		t.Errorf("want parse_error/error, got %+v", got[0])
	}
}

func TestDiagnose_LoadNotFound(t *testing.T) {
	src := `load("missing.star", "x")
print(x)
`
	got := diag(t, src, map[string]SymbolInfo{})
	var loadErrs, unresolved int
	for _, d := range got {
		switch d.Code {
		case "load_not_found":
			loadErrs++
		case "unresolved_name":
			unresolved++
		}
	}
	if loadErrs != 1 {
		t.Errorf("want 1 load_not_found, got %d (all: %+v)", loadErrs, got)
	}
	if unresolved != 0 {
		t.Errorf("want 0 unresolved_name, got %d", unresolved)
	}
}

func TestDiagnose_LoadFound(t *testing.T) {
	src := `load("helpers.star", "fetch")
print(fetch())
`
	symbols := map[string]SymbolInfo{
		"fetch": {File: "/path/scripts/helpers.star", Kind: "function"},
	}
	got := diag(t, src, symbols)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}

func TestDiagnose_UnresolvedName(t *testing.T) {
	src := `print(undefined_thing)`
	got := diag(t, src, nil)
	var found bool
	for _, d := range got {
		if d.Code == "unresolved_name" && d.Severity == "warning" && strings.Contains(d.Message, "undefined_thing") {
			found = true
		}
	}
	if !found {
		t.Errorf("want unresolved_name for undefined_thing, got %+v", got)
	}
}

func TestDiagnose_BuiltinResolves(t *testing.T) {
	src := `print(len([1,2,3]))`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}

func TestDiagnose_SwitchyardGlobalResolves(t *testing.T) {
	src := `state("light.x").value`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}

func TestDiagnose_ParamShadowsGlobal(t *testing.T) {
	src := `x = 1
def foo(x):
    return x
`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}

func TestDiagnose_ForwardReference(t *testing.T) {
	src := `def a():
    return b()

def b():
    return 1
`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}

func TestDiagnose_ComprehensionScope(t *testing.T) {
	src := `r = [y for y in range(10)]`
	got := diag(t, src, nil)
	if len(got) != 0 {
		t.Errorf("want 0 diagnostics, got %v", got)
	}
}
