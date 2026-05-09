package api_test

import (
	"testing"

	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/auth"
)

func TestRegisterWidgetPackProcedures(t *testing.T) {
	type entry struct {
		Procedure string
		Action    auth.Action
	}
	var got []entry
	api.RegisterWidgetPackProcedures(func(proc string, a auth.Action, _ func(any) auth.Target) {
		got = append(got, entry{Procedure: proc, Action: a})
	})
	want := []string{"Install", "Uninstall", "List", "Watch"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, m := range want {
		if got[i].Action.Method != map[string]string{
			"Install": "install", "Uninstall": "uninstall", "List": "list", "Watch": "watch",
		}[m] {
			t.Errorf("entry[%d] method = %q", i, got[i].Action.Method)
		}
	}
}
