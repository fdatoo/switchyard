package bridge

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client talks to a single Philips Hue bridge over CLIP v2. Safe for
// concurrent use by multiple goroutines.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
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
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// ListLights returns every light resource on the bridge.
func (c *Client) ListLights(ctx context.Context) ([]Light, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/clip/v2/resource/light", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("hue-application-key", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // body read fully in success/error paths
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hue: list lights: status %d: %s", resp.StatusCode, body)
	}
	var out listLightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("hue: decode list lights: %w", err)
	}
	return out.Data, nil
}

// SetLight applies an update to one light resource. Returns nil on 2xx,
// an error otherwise.
func (c *Client) SetLight(ctx context.Context, id string, update LightUpdate) error {
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
	resp, err := c.httpClient.Do(req)
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
