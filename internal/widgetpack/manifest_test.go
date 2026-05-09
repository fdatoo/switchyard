package widgetpack_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

const validManifest = `
@ModuleInfo { minPklVersion = "0.27.0" }
amends "switchyard:widgets"

manifest = new PackManifest {
  name = "bar-widgets"
  version = "1.0.0"
  protocol = "v1"
  sdkVersion = "1.0.0"
  bundle = "bundle.js"
  bundleHash = "sha256:abc"
  classes = new { "BarChart"; "PieChart" }
}
`

func TestEvalManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	if err := os.WriteFile(path, []byte(validManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := widgetpack.EvalManifest(context.Background(), path)
	if err != nil {
		t.Fatalf("EvalManifest: %v", err)
	}
	if m.Name != "bar-widgets" {
		t.Errorf("Name = %q", m.Name)
	}
	if len(m.Classes) != 2 {
		t.Errorf("Classes len = %d", len(m.Classes))
	}
}

func TestEvalManifest_MissingRequired(t *testing.T) {
	bad := strings.Replace(validManifest, "name = \"bar-widgets\"", "", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := widgetpack.EvalManifest(context.Background(), path); err == nil {
		t.Error("expected EvalManifest to fail on missing name")
	}
}

func TestEvalManifest_BadProtocol(t *testing.T) {
	bad := strings.Replace(validManifest, "protocol = \"v1\"", "protocol = \"v2\"", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := widgetpack.EvalManifest(context.Background(), path); err == nil {
		t.Error("expected EvalManifest to reject non-v1 protocol")
	}
}

func TestEvalManifest_BadBundleHash(t *testing.T) {
	bad := strings.Replace(validManifest, "bundleHash = \"sha256:abc\"", "bundleHash = \"md5:abc\"", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := widgetpack.EvalManifest(context.Background(), path); err == nil {
		t.Error("expected EvalManifest to reject non-sha256 bundleHash")
	}
}

func TestEvalManifest_NullManifest(t *testing.T) {
	src := `
@ModuleInfo { minPklVersion = "0.27.0" }
amends "switchyard:widgets"
// no manifest = ... assignment; defaults to null
`
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := widgetpack.EvalManifest(context.Background(), path); err == nil {
		t.Error("expected error when manifest property is null")
	}
}

func TestEvalManifest_OptionalFields(t *testing.T) {
	src := strings.Replace(validManifest,
		"classes = new { \"BarChart\"; \"PieChart\" }",
		"classes = new { \"BarChart\" }\n  description = \"Bar charts\"\n  homepage = \"https://example.org\"\n  license = \"MIT\"",
		1)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := widgetpack.EvalManifest(context.Background(), path)
	if err != nil {
		t.Fatalf("EvalManifest: %v", err)
	}
	if m.Description != "Bar charts" || m.Homepage != "https://example.org" || m.License != "MIT" {
		t.Errorf("optional fields = (%q, %q, %q)", m.Description, m.Homepage, m.License)
	}
}
