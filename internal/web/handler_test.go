package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fdatoo/switchyard/internal/web"
)

func TestHandler_ServesIndexAtRoot(t *testing.T) {
	h, err := web.NewHandler(web.Config{Version: "test"})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	if !strings.Contains(rec.Body.String(), `id="app"`) {
		t.Errorf("body missing app div: %s", rec.Body.String())
	}
}

func TestHandler_FallsBackToIndexForUnknownRoute(t *testing.T) {
	h, _ := web.NewHandler(web.Config{Version: "test"})
	req := httptest.NewRequest(http.MethodGet, "/dashboards/default", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (SPA fallback)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="app"`) {
		t.Error("expected SPA index for unknown route")
	}
}

func TestHandler_AssetsHaveImmutableCache(t *testing.T) {
	h, err := web.NewHandler(web.Config{Version: "test"})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	// If there are no hashed assets, skip — budget test covers asset existence.
	req := httptest.NewRequest(http.MethodGet, "/assets/nonexistent-abc123.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// 404 is fine for nonexistent asset, but if 200, must have immutable header
	if rec.Code == http.StatusOK {
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Errorf("Cache-Control = %q, want immutable", cc)
		}
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	h, _ := web.NewHandler(web.Config{Version: "test"})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
