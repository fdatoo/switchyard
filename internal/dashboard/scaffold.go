package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldDashboard creates the two-file Pkl split for a new dashboard in a
// dashboards directory.
func ScaffoldDashboard(dashboardDir, slug, title string) error {
	if err := os.MkdirAll(dashboardDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir: %w", err)
	}

	sourcePath := filepath.Join(dashboardDir, slug+".pkl")
	layoutPath := filepath.Join(dashboardDir, slug+".layout.pkl")

	sourceContent := fmt.Sprintf(`import "switchyard:dashboards" as d
import "%s.layout.pkl" as layout

dashboard = new d.Dashboard {
  slug = %q
  title = %q
  grid = layout.grid
  widgets = layout.widgets
}
`, slug, slug, title)

	const layoutContent = `import "switchyard:widgets" as widgetmod

// Auto-generated layout — do not edit manually.
grid: widgetmod.Grid = new { columns = 12; rowHeight = 60 }
widgets: Listing<widgetmod.WidgetInstance> = new {}
`

	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o644); err != nil {
		return fmt.Errorf("scaffold: write source: %w", err)
	}
	if err := os.WriteFile(layoutPath, []byte(layoutContent), 0o644); err != nil {
		return fmt.Errorf("scaffold: write layout: %w", err)
	}
	return nil
}
