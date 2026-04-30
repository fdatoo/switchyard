package web

import (
	"net/http"
	"strings"
)

// WidgetsHandler serves files from the widget pack asset directory.
// URL pattern: /widgets/<pack>/<version>/<file>
type WidgetsHandler struct {
	packDir string // root directory for installed pack assets
}

// NewWidgetsHandler creates a handler for serving widget pack assets.
func NewWidgetsHandler(packDir string) *WidgetsHandler {
	return &WidgetsHandler{packDir: packDir}
}

// ServeHTTP serves widget pack assets from disk.
func (h *WidgetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Strip the /widgets/ prefix to get the relative asset path.
	path := strings.TrimPrefix(r.URL.Path, "/widgets/")
	if path == "" || strings.Contains(path, "..") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, h.packDir+"/"+path)
}

var _ http.Handler = (*WidgetsHandler)(nil)
