package script_test

import (
	"context"
	"testing"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	"github.com/fdatoo/gohome/internal/script"
)

func TestReload_SwapsAtomically(t *testing.T) {
	snap1 := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{{Name: "a", Handler: "x = 1"}}}
	eng, _ := newTestEngine(t, snap1)

	if _, err := eng.Get("a"); err != nil {
		t.Fatal(err)
	}

	snap2 := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{{Name: "b", Handler: "x = 1"}}}
	if err := eng.Reload(snap2); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Get("a"); err == nil {
		t.Fatal("want not-found for a after reload")
	}
	if _, err := eng.Get("b"); err != nil {
		t.Fatal(err)
	}
	_ = context.Background
}

func TestReload_RejectsBadConfig(t *testing.T) {
	snap1 := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{{Name: "a", Handler: "x = 1"}}}
	eng, _ := newTestEngine(t, snap1)
	bad := &configpb.ConfigSnapshot{Scripts: []*configpb.ScriptConfig{{Name: "b", Handler: "def (:"}}}
	if err := eng.Reload(bad); err == nil {
		t.Fatal("want error")
	}
	if _, err := eng.Get("a"); err != nil {
		t.Fatal("original lost after failed reload")
	}
}

// ensure Engine.Reload is exercised independently of script.Engine being imported
var _ interface {
	Reload(*configpb.ConfigSnapshot) error
} = (*script.Engine)(nil)
