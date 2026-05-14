package web

import (
	"io/fs"
	"net/http"
	"strings"
)

// Config holds the web handler configuration.
type Config struct {
	Version string
}

// Handler serves the embedded SPA: hashed assets under /assets/ with
// long-lived immutable cache, everything else falls back to index.html.
type Handler struct {
	cfg     Config
	dist    fs.FS
	index   []byte
	assetFS http.FileSystem
}

// NewHandler constructs a Handler from the embedded app bundle.
func NewHandler(cfg Config) (*Handler, error) {
	dist, err := fs.Sub(Assets, "dist")
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(dist, "assets")
	if err != nil {
		// assets/ may not exist in minimal builds; treat as empty
		assets = dist
	}
	idx, err := renderIndex(cfg.Version, dist)
	if err != nil {
		return nil, err
	}
	return &Handler{
		cfg:     cfg,
		dist:    dist,
		index:   idx,
		assetFS: http.FS(assets),
	}, nil
}

// ServeHTTP routes the request: /assets/* gets immutable cache headers,
// everything else gets the SPA index.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.StripPrefix("/assets/", http.FileServer(h.assetFS)).ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(h.index)
}

// HealthCheck reports embed integrity.
func (h *Handler) HealthCheck() error {
	if len(h.index) == 0 {
		return fs.ErrNotExist
	}
	return nil
}

var _ http.Handler = (*Handler)(nil)
