package starlarkls_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fdatoo/switchyard/internal/starlarkls"
)

func TestExtractSymbols(t *testing.T) {
	dir := t.TempDir()
	src := `
def compute_brightness(sun, now):
    """Returns brightness 0-100 based on sun altitude."""
    return int(sun.altitude * 100)

THRESHOLD = 50
`
	err := os.WriteFile(filepath.Join(dir, "util.star"), []byte(src), 0644)
	if err != nil {
		t.Fatal(err)
	}

	syms, err := starlarkls.ExtractSymbols(dir)
	if err != nil {
		t.Fatal(err)
	}

	fn, ok := syms["compute_brightness"]
	if !ok {
		t.Fatal("missing compute_brightness")
	}
	if fn.Kind != "function" {
		t.Errorf("got kind %q, want function", fn.Kind)
	}
	if fn.Doc == "" {
		t.Error("expected non-empty doc")
	}

	g, ok := syms["THRESHOLD"]
	if !ok {
		t.Fatal("missing THRESHOLD")
	}
	if g.Kind != "global" {
		t.Errorf("got kind %q, want global", g.Kind)
	}
}

func TestExtractSymbols_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.star"), []byte("def ("), 0644) //nolint:errcheck
	// ExtractSymbols skips files with parse errors and logs them; it must not return an error itself.
	syms, err := starlarkls.ExtractSymbols(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syms) != 0 {
		t.Errorf("expected empty map, got %v", syms)
	}
}
