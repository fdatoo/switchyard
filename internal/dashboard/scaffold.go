package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldDashboard creates the two-file Pkl split for a new dashboard.
// sourcePkl = <slug>.pkl (user-owned, imports layout)
// layoutPkl = <slug>.layout.pkl (WYSIWYG-owned, auto-generated)
func ScaffoldDashboard(configDir, slug, title string) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir: %w", err)
	}

	sourcePath := filepath.Join(configDir, slug+".pkl")
	layoutPath := filepath.Join(configDir, slug+".layout.pkl")

	sourceContent := fmt.Sprintf(`amends "package://pkg.pkl-lang.org/pkl-pantry/pkl.experimental.uri@0.2.0#/URI.pkl"
import "@switchyard/dashboards.pkl" as d

slug = %q
title = %q
`, slug, title)

	const layoutContent = `amends "package://pkg.pkl-lang.org/pkl-pantry/pkl.experimental.uri@0.2.0#/URI.pkl"

// Auto-generated layout — do not edit manually.
grid { columns = 12; rowHeight = 60 }
widgets {}
`

	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o644); err != nil {
		return fmt.Errorf("scaffold: write source: %w", err)
	}
	if err := os.WriteFile(layoutPath, []byte(layoutContent), 0o644); err != nil {
		return fmt.Errorf("scaffold: write layout: %w", err)
	}
	return nil
}
