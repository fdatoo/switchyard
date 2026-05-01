package mcp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/mcp"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	deps := mcp.Deps{Version: "test"}
	return mcp.NewHTTPHandler(deps, mcp.HTTPConfig{})
}

func initializeBody() string {
	return `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`
}

func initPost(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// TestHTTPHandler_POST_CreatesSession verifies that a POST with a valid
// initialize request returns Mcp-Session-Id.
func TestHTTPHandler_POST_CreatesSession(t *testing.T) {
	h := newTestHandler(t)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp := initPost(t, srv.URL, initializeBody())
	defer resp.Body.Close()
	assert.NotEmpty(t, resp.Header.Get("Mcp-Session-Id"))
}

// TestHTTPHandler_GET_NoSession returns a non-200 for an unknown session.
func TestHTTPHandler_GET_NoSession(t *testing.T) {
	h := newTestHandler(t)
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Mcp-Session-Id", "nonexistent")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// TestHTTPHandler_SessionTimeout verifies sessions are evicted after timeout.
func TestHTTPHandler_SessionTimeout(t *testing.T) {
	deps := mcp.Deps{Version: "test"}
	h := mcp.NewHTTPHandler(deps, mcp.HTTPConfig{SessionIdleTimeout: 100 * time.Millisecond})
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Create a session.
	resp := initPost(t, srv.URL, initializeBody())
	sid := resp.Header.Get("Mcp-Session-Id")
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NotEmpty(t, sid)

	// Wait for eviction.
	time.Sleep(300 * time.Millisecond)

	// Session should now be unknown.
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("Accept", "text/event-stream")
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.NotEqual(t, http.StatusOK, resp2.StatusCode)
}
