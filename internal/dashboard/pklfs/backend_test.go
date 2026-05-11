package pklfs_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fdatoo/switchyard/internal/dashboard"
	"github.com/fdatoo/switchyard/internal/dashboard/pklfs"
)

func TestBackendReadAndSaveRoundTrip(t *testing.T) {
	root := copyFixture(t)
	be := pklfs.New(root, t.TempDir())
	ctx := context.Background()

	d, err := be.Get(ctx, "sample")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.Slug != "sample" || d.Title != "Sample Dashboard" {
		t.Fatalf("dashboard = (%q, %q), want sample Sample Dashboard", d.Slug, d.Title)
	}
	if len(d.Widgets) != 2 {
		t.Fatalf("widgets = %d, want 2", len(d.Widgets))
	}
	if d.Widgets[0].ID != "toggle" || d.Widgets[0].ClassID != "EntityToggle" {
		t.Fatalf("first widget = (%q, %q), want toggle EntityToggle", d.Widgets[0].ID, d.Widgets[0].ClassID)
	}
	if !d.WysiwygWritable || d.SourcePkl == "" || d.LayoutPkl == "" {
		t.Fatalf("source/layout flags not populated: writable=%v source=%d layout=%d", d.WysiwygWritable, len(d.SourcePkl), len(d.LayoutPkl))
	}

	sourcePath := filepath.Join(root, "dashboards", "sample.pkl")
	sourceBefore := readFile(t, sourcePath)
	layoutPath := filepath.Join(root, "dashboards", "sample.layout.pkl")

	saved, firstHash, err := be.SaveLayout(ctx, d)
	if err != nil {
		t.Fatalf("SaveLayout: %v", err)
	}
	firstLayout := readFile(t, layoutPath)
	_, secondHash, err := be.SaveLayout(ctx, saved)
	if err != nil {
		t.Fatalf("SaveLayout second pass: %v", err)
	}
	secondLayout := readFile(t, layoutPath)

	if firstHash != secondHash {
		t.Fatalf("hashes differ: %s != %s", firstHash, secondHash)
	}
	if !bytes.Equal(firstLayout, secondLayout) {
		t.Fatal("layout regeneration is not deterministic")
	}
	if !bytes.Equal(sourceBefore, readFile(t, sourcePath)) {
		t.Fatal("SaveLayout modified user-owned source Pkl")
	}
}

func TestBackendCreateListAndDeletePersistOnFilesystem(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	be := pklfs.New(root, t.TempDir())

	created, err := be.Create(ctx, "created", "Created")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Slug != "created" || !created.WysiwygWritable {
		t.Fatalf("created dashboard = %#v", created)
	}

	restarted := pklfs.New(root, t.TempDir())
	got, err := restarted.Get(ctx, "created")
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if got.Title != "Created" {
		t.Fatalf("title after restart = %q, want Created", got.Title)
	}
	metas, err := restarted.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 1 || metas[0].Slug != "created" {
		t.Fatalf("metas = %#v, want created", metas)
	}

	if err := restarted.Delete(ctx, "created", true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := restarted.Get(ctx, "created"); err != dashboard.ErrDashboardNotFound {
		t.Fatalf("Get after delete err = %v, want ErrDashboardNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(root, "dashboards", "created.layout.pkl")); !os.IsNotExist(err) {
		t.Fatalf("layout file still exists or stat failed: %v", err)
	}
}

func copyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/config")); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return root
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
