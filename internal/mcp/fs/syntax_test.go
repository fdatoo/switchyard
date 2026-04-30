package fs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/mcp/fs"
)

func TestCheckSyntax_PklOK(t *testing.T) {
	require.NoError(t, fs.CheckSyntax("config.pkl", []byte(`x = 1`)))
}

func TestCheckSyntax_PklBroken(t *testing.T) {
	err := fs.CheckSyntax("config.pkl", []byte(`x = {`))
	require.Error(t, err)
	var se *fs.SyntaxError
	require.ErrorAs(t, err, &se)
	require.Equal(t, "config.pkl", se.Path)
}

func TestCheckSyntax_StarlarkOK(t *testing.T) {
	require.NoError(t, fs.CheckSyntax("script.star", []byte(`def f(): return 1`)))
}

func TestCheckSyntax_StarlarkBroken(t *testing.T) {
	err := fs.CheckSyntax("script.star", []byte(`def f(`))
	require.Error(t, err)
	var se *fs.SyntaxError
	require.ErrorAs(t, err, &se)
	require.NotZero(t, se.Line)
}

func TestCheckSyntax_UnsupportedExtension(t *testing.T) {
	err := fs.CheckSyntax("README.md", []byte(`# hi`))
	require.ErrorIs(t, err, fs.ErrUnsupportedExtension)
}
