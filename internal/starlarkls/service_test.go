package starlarkls_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	starlarkpb "github.com/fdatoo/switchyard/gen/switchyard/starlarkls/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/starlarkls/v1/starlarklsv1connect"
	"github.com/fdatoo/switchyard/internal/starlarkls"
)

func setupService(t *testing.T) (starlarklsv1connect.StarlarkLsServiceHandler, string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "util.star"), []byte(`
def compute_brightness(sun, now):
    """Returns brightness 0-100."""
    return int(sun.altitude * 100)
`), 0644) //nolint:errcheck
	syms, err := starlarkls.ExtractSymbols(dir)
	if err != nil {
		t.Fatal(err)
	}
	return starlarkls.NewService(syms, dir), dir
}

func TestComplete_GlobalSymbol(t *testing.T) {
	svc, dir := setupService(t)
	resp, err := svc.Complete(context.Background(), connect.NewRequest(&starlarkpb.CompleteRequest{
		FilePath: filepath.Join(dir, "util.star"),
		Source:   "compute_b",
		Line:     1,
		Col:      9,
	}))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range resp.Msg.Items {
		if item.Label == "compute_brightness" {
			found = true
		}
	}
	if !found {
		t.Error("expected compute_brightness in completion items")
	}
}

func TestHover_KnownSymbol(t *testing.T) {
	svc, dir := setupService(t)
	resp, err := svc.Hover(context.Background(), connect.NewRequest(&starlarkpb.HoverRequest{
		FilePath: filepath.Join(dir, "util.star"),
		Source:   "compute_brightness(sun, now)",
		Line:     1,
		Col:      5,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.Markdown == "" {
		t.Error("expected non-empty hover markdown")
	}
}

func TestLookupSymbol_Found(t *testing.T) {
	svc, _ := setupService(t)
	resp, err := svc.LookupSymbol(context.Background(), connect.NewRequest(&starlarkpb.LookupSymbolRequest{
		Name: "compute_brightness",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.Kind != "function" {
		t.Errorf("want function, got %q", resp.Msg.Kind)
	}
}

func TestLookupSymbol_NotFound(t *testing.T) {
	svc, _ := setupService(t)
	_, err := svc.LookupSymbol(context.Background(), connect.NewRequest(&starlarkpb.LookupSymbolRequest{
		Name: "nonexistent",
	}))
	if err == nil {
		t.Error("expected connect NOT_FOUND error")
	}
}
