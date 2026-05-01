package bridge

import "net/http"

// SetHTTPClientForTest swaps the underlying *http.Client. Tests use this
// to inject httptest.NewTLSServer's pre-trusted client so calls to the
// fake bridge don't fail TLS verification regardless of skip-verify.
func (c *Client) SetHTTPClientForTest(h *http.Client) {
	c.httpClient = h
}
