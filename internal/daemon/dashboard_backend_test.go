package daemon

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestDashboardBackend_WidgetCatalog_ReflectsStore(t *testing.T) {
	store := widgetpack.NewStore(t.TempDir())
	if err := store.Load(context.Background()); err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if err := store.Add(context.Background(), widgetpack.InstalledPack{
		Name:    "bar-widgets",
		Version: "1.0.0",
		SHA256:  "sha256:abc",
		Classes: []string{"BarChart", "PieChart"},
	}); err != nil {
		t.Fatalf("store.Add: %v", err)
	}

	be := newDashboardBackend(t.TempDir(), t.TempDir(), store)
	classes, err := be.WidgetCatalog(context.Background())
	if err != nil {
		t.Fatalf("WidgetCatalog: %v", err)
	}

	// Must include 8 builtins + 2 pack classes.
	if len(classes) < 10 {
		t.Errorf("expected at least 10 classes (8 builtins + 2 pack), got %d", len(classes))
	}

	// Find the pack class "bar-widgets/BarChart".
	var found bool
	for _, c := range classes {
		if c.ClassID == "bar-widgets/BarChart" {
			found = true
			if c.IsBuiltin {
				t.Errorf("bar-widgets/BarChart should not be marked builtin")
			}
			if c.PackName != "bar-widgets" {
				t.Errorf("PackName = %q, want bar-widgets", c.PackName)
			}
			if c.PackVersion != "1.0.0" {
				t.Errorf("PackVersion = %q, want 1.0.0", c.PackVersion)
			}
			wantURL := "/widgets/bar-widgets/1.0.0/bundle.js?h=sha256:abc"
			if c.BundleURL != wantURL {
				t.Errorf("BundleURL = %q, want %q", c.BundleURL, wantURL)
			}
			break
		}
	}
	if !found {
		t.Error("bar-widgets/BarChart not found in catalog")
	}
}

func TestDashboardBackend_WidgetCatalog_NilStore(t *testing.T) {
	be := newDashboardBackend(t.TempDir(), t.TempDir(), nil)
	classes, err := be.WidgetCatalog(context.Background())
	if err != nil {
		t.Fatalf("WidgetCatalog: %v", err)
	}
	// Should still return the 8 builtins.
	if len(classes) != 8 {
		t.Errorf("expected 8 builtin classes with nil store, got %d", len(classes))
	}
}
