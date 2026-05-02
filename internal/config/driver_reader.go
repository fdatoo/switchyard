package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/apple/pkl-go/pkl"
)

// driverModuleReader serves driver:<name> Pkl modules from
// <root>/<name>/manifest.pkl.
type driverModuleReader struct{ root string }

func (r *driverModuleReader) Scheme() string            { return "driver" }
func (r *driverModuleReader) IsGlobbable() bool         { return false }
func (r *driverModuleReader) HasHierarchicalUris() bool { return false }
func (r *driverModuleReader) IsLocal() bool             { return true }
func (r *driverModuleReader) ListElements(_ url.URL) ([]pkl.PathElement, error) {
	return nil, nil
}

func (r *driverModuleReader) Read(u url.URL) (string, error) {
	name := u.Opaque
	if !validDriverName(name) {
		return "", fmt.Errorf("invalid driver name %q in driver:%s", name, name)
	}
	path := filepath.Join(r.root, name, "manifest.pkl")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("driver %q: manifest not found at %s", name, path)
	}
	return string(data), nil
}

// validDriverName returns true for non-empty strings of length 1..64 made up
// of lowercase ASCII letters, digits, '-', and '_'. Used to keep arbitrary
// path components out of driver: URIs.
func validDriverName(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
