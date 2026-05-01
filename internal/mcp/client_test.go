package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/mcp"
)

func TestNewClient_HTTPScheme(t *testing.T) {
	var observed http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		observed = r.Header.Clone()
	}))
	defer srv.Close()

	c, err := mcp.NewClient(mcp.ClientOptions{
		EndpointURL: srv.URL,
		SessionID:   "test-session",
	})
	require.NoError(t, err)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL+"/test", nil)
	req.Header.Set("x-switchyard-source", "mcp")
	req.Header.Set("x-switchyard-mcp-session", "test-session")
	resp, err := c.HTTPClient().Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	require.Equal(t, "mcp", observed.Get("x-switchyard-source"))
	require.Equal(t, "test-session", observed.Get("x-switchyard-mcp-session"))
}

func TestSetToolHeader(t *testing.T) {
	opt := mcp.SetToolHeader("switchyard__get_state")
	require.NotNil(t, opt)
}
