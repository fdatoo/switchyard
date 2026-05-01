//go:build integration

package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/daemon"
	"github.com/fdatoo/switchyard/internal/observability"
)

// moduleRoot walks up from the test's working directory to find go.mod,
// returning the directory that contains it. This is needed because Go sets
// the test working directory to the package directory, not the module root.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root (no go.mod found)")
		}
		dir = parent
	}
}

func TestE2E_MCPTools(t *testing.T) {
	// Just verify that 'gohome mcp tools' works without a daemon.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gohome := filepath.Join(moduleRoot(t), "dist", "gohome")
	cmd := exec.CommandContext(ctx, gohome, "mcp", "tools", "--json")
	out, err := cmd.Output()
	require.NoError(t, err)
	require.Contains(t, string(out), "gohome__get_state")
	require.Contains(t, string(out), "gohome__write_config_file")
}

func TestE2E_MCPServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// ── 1. Start a real gohomed daemon in-process ──────────────────────────────
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	metrics := observability.NewMetrics()
	adminPort := freeTCPPort(t)

	d := daemon.New(daemon.Config{
		DataDir:   dir,
		LogLevel:  slog.LevelInfo,
		LogFormat: "json",
		AdminPort: adminPort,
	}, logger, metrics)

	daemonCtx, daemonCancel := context.WithCancel(ctx)
	defer daemonCancel()
	done := make(chan error, 1)
	go func() { done <- d.Run(daemonCtx) }()

	// ── 2. Wait for daemon to be healthy ──────────────────────────────────────
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", adminPort)
	deadline := time.Now().Add(30 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL) //nolint:noctx
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		daemonCancel()
		<-done
		t.Fatal("daemon did not report ready within 30s")
	}

	// ── 3. Exec 'gohome mcp serve' as a real subprocess ───────────────────────
	// The daemon always creates its API socket at <DataDir>/gohomed.sock.
	sockPath := filepath.Join(dir, "gohomed.sock")
	gohome := filepath.Join(moduleRoot(t), "dist", "gohome")
	cmd := exec.CommandContext(ctx, gohome, "mcp", "serve",
		"--endpoint", "unix://"+sockPath)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	// ── 4. Connect an MCP SDK client to the subprocess via stdio pipes ─────────
	transport := &sdk.IOTransport{
		Reader: stdout,
		Writer: stdin,
	}
	impl := &sdk.Implementation{Name: "test-e2e", Version: "0"}
	mcpClient := sdk.NewClient(impl, nil)

	// Give the subprocess up to 5s to initialise (it dials the daemon socket).
	connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Second)
	defer connectCancel()
	cs, err := mcpClient.Connect(connectCtx, transport, nil)
	require.NoError(t, err, "MCP client connect")
	t.Cleanup(func() { _ = cs.Close() })

	// ── 5. Assertions ──────────────────────────────────────────────────────────

	// 5a. ListTools — expect exactly 12 tools.
	toolsResult, err := cs.ListTools(ctx, nil)
	require.NoError(t, err, "ListTools")
	assert.Len(t, toolsResult.Tools, 12, "expected 12 tools")

	// 5b. ListResources — expect 1 resource (gohome://entities/).
	resourcesResult, err := cs.ListResources(ctx, nil)
	require.NoError(t, err, "ListResources")
	assert.Len(t, resourcesResult.Resources, 1, "expected 1 resource")
	if len(resourcesResult.Resources) > 0 {
		assert.Equal(t, "gohome://entities/", resourcesResult.Resources[0].URI, "resource URI")
	}

	// 5c. ListResourceTemplates — expect 2 templates.
	tmplResult, err := cs.ListResourceTemplates(ctx, nil)
	require.NoError(t, err, "ListResourceTemplates")
	assert.Len(t, tmplResult.ResourceTemplates, 2, "expected 2 resource templates")

	// 5d. CallTool gohome__list_entities — empty daemon has no entities.
	listRes, err := cs.CallTool(ctx, &sdk.CallToolParams{
		Name:      "gohome__list_entities",
		Arguments: map[string]any{},
	})
	require.NoError(t, err, "CallTool gohome__list_entities")
	assert.False(t, listRes.IsError, "gohome__list_entities should not error")
	require.NotEmpty(t, listRes.Content, "gohome__list_entities content")
	listText, ok := listRes.Content[0].(*sdk.TextContent)
	require.True(t, ok, "gohome__list_entities content[0] should be TextContent")
	var listOut map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(listText.Text), &listOut), "parse list_entities JSON")
	assert.Contains(t, listOut, "entities", "entities key present")
	assert.Contains(t, listOut, "next_cursor", "next_cursor key present")

	// 5e. CallTool gohome__eval_starlark — evaluates log(1 + 1), expects "2" in stdout.
	// The Starlark runtime captures log() calls in the stdout field; the built-in
	// print() goes to the process stderr and is not captured in the response.
	evalRes, err := cs.CallTool(ctx, &sdk.CallToolParams{
		Name:      "gohome__eval_starlark",
		Arguments: map[string]any{"source": "log(1 + 1)"},
	})
	require.NoError(t, err, "CallTool gohome__eval_starlark")
	assert.False(t, evalRes.IsError, "gohome__eval_starlark should not error")
	require.NotEmpty(t, evalRes.Content, "gohome__eval_starlark content")
	evalText, ok := evalRes.Content[0].(*sdk.TextContent)
	require.True(t, ok, "gohome__eval_starlark content[0] should be TextContent")
	var evalOut map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(evalText.Text), &evalOut), "parse eval_starlark JSON")
	var stdoutStr string
	require.NoError(t, json.Unmarshal(evalOut["stdout"], &stdoutStr), "parse stdout field")
	assert.Contains(t, stdoutStr, "2", "stdout should contain \"2\"")
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}
