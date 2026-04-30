package starlark_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	starlarkgo "go.starlark.net/starlark"

	ghs "github.com/fdatoo/gohome/internal/starlark"
)

func newTestRuntimeWithDir(t *testing.T, dir string) *ghs.Runtime {
	t.Helper()
	return ghs.NewRuntime(fakeState{}, &fakeDispatcher{}, &fakeAppender{}, nil, dir, nil)
}

func TestLoader_HappyPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.star"), []byte(`def greet(): return "hello"`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	res, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//lib.star", "greet")
result = greet()`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res
}

func TestLoader_PathTraversalRejected(t *testing.T) {
	rt := newTestRuntimeWithDir(t, t.TempDir())
	_, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//../../etc/passwd", "x")`, starlarkgo.StringDict{})
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestLoader_CircularDependency(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.star"), []byte(`load("//b.star", "b"); a = b`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.star"), []byte(`load("//a.star", "a"); b = a`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	_, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//a.star", "a")`, starlarkgo.StringDict{})
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestLoader_CacheInvalidation(t *testing.T) {
	dir := t.TempDir()
	libPath := filepath.Join(dir, "lib.star")
	if err := os.WriteFile(libPath, []byte(`val = "v1"`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	res1, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//lib.star", "val"); result = val`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	_ = res1

	if err := os.WriteFile(libPath, []byte(`val = "v2"`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.InvalidateModuleCache()
	res2, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//lib.star", "val"); result = val`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}
	_ = res2
}

func TestLoader_NonSlashSlashRejected(t *testing.T) {
	rt := newTestRuntimeWithDir(t, t.TempDir())
	_, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("./relative.star", "x")`, starlarkgo.StringDict{})
	if err == nil {
		t.Fatal("expected error for non-// load path")
	}
}

func TestLoader_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.star"), []byte(`val = 42`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = rt.Execute(t.Context(), ghs.KindScript,
				`load("//lib.star", "val")`, starlarkgo.StringDict{})
		}()
	}
	wg.Wait()
}
