package bridge

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ErrAuthRevoked indicates the bridge has rejected the API key three or
// more times within 60 seconds. The driver treats this as fatal: the
// only recovery is human intervention (re-pair via button press).
var ErrAuthRevoked = errors.New("hue: api key rejected by bridge (re-pair required)")

const (
	authFailureThreshold = 3
	authFailureWindow    = 60 * time.Second
)

type authFailureTracker struct {
	mu         sync.Mutex
	timestamps []time.Time
}

// record adds a fresh failure at `now` and prunes anything older than the window.
func (t *authFailureTracker) record(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := now.Add(-authFailureWindow)
	t.timestamps = append(t.timestamps, now)
	n := 0
	for _, ts := range t.timestamps {
		if ts.After(cutoff) {
			t.timestamps[n] = ts
			n++
		}
	}
	t.timestamps = t.timestamps[:n]
}

// revoked reports whether the live failure count meets the threshold.
func (t *authFailureTracker) revoked(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := now.Add(-authFailureWindow)
	count := 0
	for _, ts := range t.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}
	return count >= authFailureThreshold
}

// Client talks to a single Philips Hue bridge over CLIP v2. Safe for
// concurrent use by multiple goroutines.
type Client struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	authTracker authFailureTracker
	limiter     *rateLimiter
}

// Option is a functional option for New.
type Option func(*Client)

// WithHTTPClient replaces the default *http.Client. Useful in tests to inject
// an httptest.Server's pre-trusted client so TLS verification succeeds against
// a self-signed certificate.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// New constructs a Client. address is "<host>" or "<host>:<port>" — the
// CLIP v2 API is always HTTPS, so no scheme. apiKey is the bridge
// application key. tlsSkipVerify defaults to true in production because the
// bridge ships a self-signed cert.
func New(address, apiKey string, tlsSkipVerify bool, opts ...Option) (*Client, error) {
	if address == "" {
		return nil, fmt.Errorf("bridge address is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	c := &Client{
		baseURL: "https://" + address,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: tlsSkipVerify}, //nolint:gosec // bridge ships self-signed cert
			},
		},
		limiter: newRateLimiter(10, 10),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// do issues an HTTP request through the client's transport, applying the
// pre-call auth-revoked short-circuit and recording 401s. Caller closes
// the response body.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	if err := c.limiter.wait(req.Context()); err != nil {
		return nil, err
	}
	if c.authTracker.revoked(time.Now()) {
		return nil, ErrAuthRevoked
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		c.authTracker.record(time.Now())
		// The caller still gets the response so they can read the body
		// for diagnostics — but most callers will treat 401 as an error
		// regardless.
	}
	return resp, nil
}

// recordAuthFailureAt is a test helper exported within the package for
// driving the failure window from tests.
func (c *Client) recordAuthFailureAt(t time.Time) {
	c.authTracker.record(t)
}

// getJSON issues a GET to path and decodes the JSON response into v.
// Applies the rate limiter, auth tracking, and 10s timeout.
func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // body read fully in success/error paths
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hue: %s: status %d: %s", path, resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// ListLights returns every light resource on the bridge.
func (c *Client) ListLights(ctx context.Context) ([]Light, error) {
	var out listLightsResponse
	if err := c.getJSON(ctx, "/clip/v2/resource/light", &out); err != nil {
		return nil, fmt.Errorf("hue: list lights: %w", err)
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("hue: list lights: %s", out.Errors[0].Description)
	}
	return out.Data, nil
}

// SetLight applies an update to one light resource. Returns nil on 2xx,
// an error otherwise.
func (c *Client) SetLight(ctx context.Context, id string, update LightUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	body, err := json.Marshal(update)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/clip/v2/resource/light/"+id, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // body read fully in success/error paths
	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hue: set light %s: status %d: %s", id, resp.StatusCode, respBody)
	}
	return nil
}
