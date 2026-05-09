package widgetpack_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestBundleHandler_GET(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "bar/1.0.0/bundle.js"), []byte("export const X=1;"))
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	_ = store.Add(context.Background(), widgetpack.InstalledPack{
		Name: "bar", Version: "1.0.0", SHA256: "sha256:hashval",
	})
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/widgets/bar/1.0.0/bundle.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control=%q", got)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/javascript" {
		t.Errorf("Content-Type=%q", got)
	}
	if got := resp.Header.Get("ETag"); got != `"sha256:hashval"` {
		t.Errorf("ETag=%q", got)
	}
}

func TestBundleHandler_PathTraversal(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "bar/1.0.0/bundle.js"), []byte("ok"))
	mustWrite(t, filepath.Join(root, "secret.txt"), []byte("secret"))
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	_ = store.Add(context.Background(), widgetpack.InstalledPack{Name: "bar", Version: "1.0.0", SHA256: "sha256:x"})
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/widgets/bar/1.0.0/../../secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("path traversal not blocked")
	}
}

func TestBundleHandler_UnknownPack(t *testing.T) {
	root := t.TempDir()
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/widgets/unknown/1.0.0/bundle.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status=%d, want 404", resp.StatusCode)
	}
}

func TestBundleHandler_MethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, err := http.NewRequest("POST", srv.URL+"/widgets/bar/1.0.0/bundle.js", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Errorf("status=%d, want 405", resp.StatusCode)
	}
}

func TestBundleHandler_IfNoneMatch(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "bar/1.0.0/bundle.js"), []byte("ok"))
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	_ = store.Add(context.Background(), widgetpack.InstalledPack{Name: "bar", Version: "1.0.0", SHA256: "sha256:hashval"})
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, err := http.NewRequest("GET", srv.URL+"/widgets/bar/1.0.0/bundle.js", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("If-None-Match", `"sha256:hashval"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 304 {
		t.Errorf("status=%d, want 304", resp.StatusCode)
	}
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}
