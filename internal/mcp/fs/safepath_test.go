package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/mcp/fs"
)

func TestResolve_OK(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "automations"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "automations", "lights.pkl"), []byte("x=1"), 0o644))
	got, err := fs.Resolve(root, "automations/lights.pkl")
	require.NoError(t, err)
	// Resolve follows symlinks (e.g. /var/folders → /private/var/folders on macOS).
	resolvedRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(resolvedRoot, "automations", "lights.pkl"), got)
}

func TestResolve_RejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	_, err := fs.Resolve(root, "../etc/passwd")
	require.ErrorIs(t, err, fs.ErrPathEscape)
}

func TestResolve_RejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	_, err := fs.Resolve(root, "/etc/passwd")
	require.ErrorIs(t, err, fs.ErrPathEscape)
}

func TestResolve_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "link.txt")))
	_, err := fs.Resolve(root, "link.txt")
	require.ErrorIs(t, err, fs.ErrPathEscape)
}

func TestResolve_AllowsNonexistentTarget(t *testing.T) {
	root := t.TempDir()
	got, err := fs.Resolve(root, "automations/new.pkl")
	require.NoError(t, err)
	// Resolve uses the EvalSymlinks-resolved root (e.g. /var/folders → /private/var/folders on macOS).
	resolvedRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(resolvedRoot, "automations", "new.pkl"), got)
}
