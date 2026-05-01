package regen_test

import (
	"strings"
	"testing"

	"github.com/fdatoo/switchyard/internal/dashboard"
	"github.com/fdatoo/switchyard/internal/dashboard/regen"
)

func TestRender_EmptyDashboard(t *testing.T) {
	d := &dashboard.DashboardData{
		Slug:  "empty",
		Title: "Empty",
		Grid:  dashboard.GridData{Columns: 12, RowHeight: 60},
	}
	out, err := regen.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "grid") {
		t.Error("missing grid declaration")
	}
	if !strings.Contains(s, "widgets") {
		t.Error("missing widgets block")
	}
}

func TestRender_SingleWidget(t *testing.T) {
	d := &dashboard.DashboardData{
		Slug: "test",
		Grid: dashboard.GridData{Columns: 12, RowHeight: 60},
		Widgets: []dashboard.WidgetData{
			{
				ID:      "toggle-a",
				ClassID: "EntityToggle",
				Pos:     dashboard.PosData{X: 0, Y: 0, W: 3, H: 2},
				Props:   map[string]any{"entityId": "light.living_room"},
			},
		},
	}
	out, err := regen.Render(d)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `id = "toggle-a"`) {
		t.Errorf("missing widget id in output: %s", s)
	}
	if !strings.Contains(s, `widgetClass = "EntityToggle"`) {
		t.Error("missing widgetClass in output")
	}
}

func TestRender_Deterministic(t *testing.T) {
	d := &dashboard.DashboardData{
		Slug: "determ",
		Grid: dashboard.GridData{Columns: 12, RowHeight: 60},
		Widgets: []dashboard.WidgetData{
			{ID: "b", ClassID: "Gauge", Pos: dashboard.PosData{W: 2, H: 2}},
			{ID: "a", ClassID: "EntityToggle", Pos: dashboard.PosData{W: 1, H: 1}},
		},
	}
	first, _ := regen.Render(d)
	for i := 0; i < 5; i++ {
		out, _ := regen.Render(d)
		if string(out) != string(first) {
			t.Fatalf("non-deterministic on iteration %d", i)
		}
	}
}
