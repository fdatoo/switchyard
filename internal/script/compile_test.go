package script_test

import (
	"testing"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	"github.com/fdatoo/gohome/internal/script"
)

func minimalScripts(t *testing.T, scripts []*configpb.ScriptConfig) *configpb.ConfigSnapshot {
	t.Helper()
	return &configpb.ConfigSnapshot{Scripts: scripts}
}

func TestCompile_Empty(t *testing.T) {
	snap := minimalScripts(t, nil)
	m, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("got %d", len(m))
	}
}

func TestCompile_Ok(t *testing.T) {
	snap := minimalScripts(t, []*configpb.ScriptConfig{
		{Name: "greet", Handler: "def main(params): pass"},
	})
	m, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["greet"]; !ok {
		t.Fatal("want greet")
	}
}

func TestCompile_DuplicateName(t *testing.T) {
	snap := minimalScripts(t, []*configpb.ScriptConfig{
		{Name: "x", Handler: "def main(params): pass"},
		{Name: "x", Handler: "def main(params): pass"},
	})
	if _, err := script.CompileScripts(snap); err == nil {
		t.Fatal("want dup error")
	}
}

func TestCompile_BadHandler(t *testing.T) {
	snap := minimalScripts(t, []*configpb.ScriptConfig{
		{Name: "broken", Handler: "def main(:"},
	})
	if _, err := script.CompileScripts(snap); err == nil {
		t.Fatal("want parse error")
	}
}

func TestCompile_DefaultCoerce(t *testing.T) {
	snap := minimalScripts(t, []*configpb.ScriptConfig{
		{
			Name:    "withparam",
			Handler: "def main(params): pass",
			Params: []*configpb.ScriptParam{
				{Name: "n", Type: configpb.ScriptParam_TYPE_INT, Required: false, Default: "7"},
			},
		},
	})
	m, err := script.CompileScripts(snap)
	if err != nil {
		t.Fatal(err)
	}
	p := m["withparam"].Params[0]
	if !p.HasDefault || p.Default.(int64) != 7 {
		t.Fatalf("default = %v hasDefault=%v", p.Default, p.HasDefault)
	}
}

func TestCompile_BadDefault(t *testing.T) {
	snap := minimalScripts(t, []*configpb.ScriptConfig{
		{
			Name:    "badDefault",
			Handler: "def main(params): pass",
			Params: []*configpb.ScriptParam{
				{Name: "n", Type: configpb.ScriptParam_TYPE_INT, Required: false, Default: "abc"},
			},
		},
	})
	if _, err := script.CompileScripts(snap); err == nil {
		t.Fatal("want coerce error")
	}
}
