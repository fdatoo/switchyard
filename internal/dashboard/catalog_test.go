package dashboard_test

import (
	"slices"
	"testing"

	"github.com/fdatoo/gohome/internal/dashboard"
)

func TestCatalog_BuiltinsOnly(t *testing.T) {
	c := dashboard.NewCatalog(nil)
	classes := c.WidgetClasses()
	wantClasses := []string{"EntityToggle", "Gauge", "LineChart", "CameraStream", "Markdown", "ScriptButton", "EntityList", "GroupCard"}
	got := make([]string, 0, len(classes))
	for _, wc := range classes {
		got = append(got, wc.ClassID)
	}
	if !slices.Equal(got, wantClasses) {
		t.Errorf("classes = %v, want %v", got, wantClasses)
	}
}

func TestCatalog_BuiltinsPlusPack(t *testing.T) {
	pack := dashboard.InstalledPack{
		Name: "bar-widgets", Version: "1.0.0",
		Classes: []dashboard.PackClass{{Name: "BarChart"}},
	}
	c := dashboard.NewCatalog([]dashboard.InstalledPack{pack})
	got := c.LookupClass("bar-widgets/BarChart")
	if got == nil {
		t.Fatal("expected to find bar-widgets/BarChart")
	}
	if got.PackName != "bar-widgets" {
		t.Errorf("PackName = %q, want bar-widgets", got.PackName)
	}
}
