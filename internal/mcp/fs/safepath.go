// Package fs provides MCP filesystem-tool helpers.
package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathEscape is returned when a path escapes the config dir.
var ErrPathEscape = errors.New("path escapes config dir")

// Resolve joins root and rel, rejects absolute rel paths and .. traversals,
// and follows symlinks to verify the final destination stays inside root.
// The target may not exist yet (write path).
func Resolve(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: absolute path %q", ErrPathEscape, rel)
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	cleanRoot, err = filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		return "", fmt.Errorf("eval root symlinks: %w", err)
	}

	joined := filepath.Join(cleanRoot, rel)
	cleaned := filepath.Clean(joined)
	if !inside(cleanRoot, cleaned) {
		return "", fmt.Errorf("%w: %q resolves outside root", ErrPathEscape, rel)
	}

	// If the target exists, verify symlinks don't escape.
	if resolved, lerr := filepath.EvalSymlinks(cleaned); lerr == nil {
		if !inside(cleanRoot, resolved) {
			return "", fmt.Errorf("%w: symlink %q escapes root", ErrPathEscape, rel)
		}
		return resolved, nil
	} else if !errors.Is(lerr, os.ErrNotExist) {
		return "", fmt.Errorf("eval symlinks: %w", lerr)
	}

	// Target doesn't exist yet — verify existing parent prefix stays inside root.
	prefix := filepath.Dir(cleaned)
	for prefix != cleanRoot && prefix != filepath.Dir(cleanRoot) {
		if resolved, lerr := filepath.EvalSymlinks(prefix); lerr == nil {
			if !inside(cleanRoot, resolved) {
				return "", fmt.Errorf("%w: parent %q escapes root", ErrPathEscape, prefix)
			}
			break
		}
		prefix = filepath.Dir(prefix)
	}
	return cleaned, nil
}

func inside(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !strings.Contains(rel, string(os.PathSeparator)+".."))
}
