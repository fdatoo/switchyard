package widgetpack

import (
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// NewBundleHandler returns an http.Handler for /widgets/<pack>/<version>/<file>.
// It serves files only for packs known to store; unknown packs return 404 even
// if the file exists on disk (e.g. mid-install staging).
func NewBundleHandler(store *Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Trim "/widgets/" prefix.
		const prefix = "/widgets/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		clean := path.Clean("/" + rel)
		parts := strings.SplitN(strings.TrimPrefix(clean, "/"), "/", 3)
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}
		pack, version, file := parts[0], parts[1], parts[2]

		p, err := store.Get(r.Context(), pack, version)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		etag := `"` + p.SHA256 + `"`
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		fullPath := filepath.Join(store.Root(), pack, version, file)
		// Re-check escape after Clean+Join.
		expectedPrefix := filepath.Join(store.Root(), pack, version) + string(filepath.Separator)
		if !strings.HasPrefix(fullPath, expectedPrefix) {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", contentTypeFor(file))
		http.ServeFile(w, r, fullPath)
	})
}

func contentTypeFor(name string) string {
	switch filepath.Ext(name) {
	case ".js", ".mjs":
		return "text/javascript"
	case ".map":
		return "application/json"
	case ".css":
		return "text/css"
	default:
		return "application/octet-stream"
	}
}
