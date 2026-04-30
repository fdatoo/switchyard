package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/mcp"
	"github.com/fdatoo/gohome/internal/mcp/tools"
)

// Since tools.Deps.Audit is *audit.Recorder (concrete type), and we need to test
// without a real daemon connection, we use nil Audit in most tests.
func newFilesTestDeps(t *testing.T, configDir string) (tools.Deps, *sdk.Server) {
	t.Helper()
	s := sdk.NewServer(testImpl, nil)
	d := tools.Deps{
		Server:    s,
		Client:    nil, // files tools don't use Client
		ConfigDir: configDir,
		MCPCaps:   mcp.MCPCaps{ReadFileMaxBytes: 1024 * 1024},
		Auth:      auth.AllowAll{},
		Audit:     nil, // no audit in unit tests
	}
	return d, s
}

func TestWriteConfigFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	d, s := newFilesTestDeps(t, dir)
	tools.Register(d)

	content := "amends \"example\"\nfoo = 1\n"
	result, err := callTool(t, s, "gohome__write_config_file", map[string]any{
		"path":    "config.pkl",
		"content": content,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, "config.pkl", out["path"])
	assert.NotEmpty(t, out["sha256_hex"])
	assert.Equal(t, float64(len(content)), out["size_bytes"])

	// Verify file was actually written
	data, rerr := os.ReadFile(filepath.Join(dir, "config.pkl"))
	require.NoError(t, rerr)
	assert.Equal(t, content, string(data))
}

func TestWriteConfigFile_PathEscape(t *testing.T) {
	dir := t.TempDir()
	d, s := newFilesTestDeps(t, dir)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__write_config_file", map[string]any{
		"path":    "../escape.pkl",
		"content": "amends \"x\"\n",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "expected IsError for path escape")
}

func TestWriteConfigFile_BadExtension(t *testing.T) {
	dir := t.TempDir()
	d, s := newFilesTestDeps(t, dir)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__write_config_file", map[string]any{
		"path":    "config.json",
		"content": `{"foo": 1}`,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "expected IsError for unsupported extension")
}

func TestWriteConfigFile_BadPklSyntax(t *testing.T) {
	dir := t.TempDir()
	d, s := newFilesTestDeps(t, dir)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__write_config_file", map[string]any{
		"path":    "config.pkl",
		"content": "{ unclosed",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "expected IsError for syntax error")

	// Verify file was NOT created
	entries, rerr := os.ReadDir(dir)
	require.NoError(t, rerr)
	// Filter out .tmp files that may be cleaned up
	for _, e := range entries {
		assert.NotEqual(t, "config.pkl", e.Name(), "config.pkl should not have been created")
	}
}

func TestReadConfigFile_HappyPath(t *testing.T) {
	dir := t.TempDir()

	// Write a file first
	content := "amends \"test\"\nbar = 2\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.pkl"), []byte(content), 0o644))

	d, s := newFilesTestDeps(t, dir)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__read_config_file", map[string]any{
		"path": "test.pkl",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, "test.pkl", out["path"])
	assert.Equal(t, content, out["content"])
	assert.NotEmpty(t, out["sha256_hex"])
}

func TestReadConfigFile_PathEscape(t *testing.T) {
	dir := t.TempDir()
	d, s := newFilesTestDeps(t, dir)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__read_config_file", map[string]any{
		"path": "../../etc/passwd",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "expected IsError for path escape")
}
