// Package mcp implements the gohome MCP server (stdio transport).
package mcp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"

	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

// ClientOptions configures the MCP-side Connect client.
type ClientOptions struct {
	EndpointURL string // unix:// or http://
	SessionID   string // ULID minted at startup
}

// Client owns the http.Client and typed service clients.
type Client struct {
	httpClient *http.Client
	baseURL    string
	sessionID  string

	System     gohomev1alpha1connect.SystemServiceClient
	Entity     gohomev1alpha1connect.EntityServiceClient
	Event      gohomev1alpha1connect.EventServiceClient
	Script     gohomev1alpha1connect.ScriptServiceClient
	Config     gohomev1alpha1connect.ConfigServiceClient
	Automation gohomev1alpha1connect.AutomationServiceClient
	Scene      gohomev1alpha1connect.SceneServiceClient
}

func NewClient(opts ClientOptions) (*Client, error) {
	u, err := url.Parse(opts.EndpointURL)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	httpc := &http.Client{Transport: buildTransport(u)}
	base := opts.EndpointURL
	if u.Scheme == "unix" {
		base = "http://gohomed"
	}
	interceptors := connect.WithInterceptors(headerInterceptor(opts.SessionID))
	c := &Client{
		httpClient: httpc,
		baseURL:    base,
		sessionID:  opts.SessionID,
		System:     gohomev1alpha1connect.NewSystemServiceClient(httpc, base, interceptors),
		Entity:     gohomev1alpha1connect.NewEntityServiceClient(httpc, base, interceptors),
		Event:      gohomev1alpha1connect.NewEventServiceClient(httpc, base, interceptors),
		Script:     gohomev1alpha1connect.NewScriptServiceClient(httpc, base, interceptors),
		Config:     gohomev1alpha1connect.NewConfigServiceClient(httpc, base, interceptors),
		Automation: gohomev1alpha1connect.NewAutomationServiceClient(httpc, base, interceptors),
		Scene:      gohomev1alpha1connect.NewSceneServiceClient(httpc, base, interceptors),
	}
	return c, nil
}

func buildTransport(u *url.URL) http.RoundTripper {
	if u.Scheme == "unix" {
		socket := u.Host + u.Path
		if socket == "" {
			socket = strings.TrimPrefix(u.Path, "/")
		}
		return &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
		}
	}
	return http.DefaultTransport
}

// headerInterceptor sets x-gohome-source: mcp and x-gohome-mcp-session on
// every outgoing unary call.
func headerInterceptor(sessionID string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("x-gohome-source", "mcp")
			req.Header().Set("x-gohome-mcp-session", sessionID)
			return next(ctx, req)
		}
	}
}

// HTTPClient exposes the underlying http.Client (used in tests).
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

// HeaderOption carries a header key/value to be applied to an outgoing request.
type HeaderOption struct {
	Key   string
	Value string
}

// SetToolHeader returns a HeaderOption that adds the tool header.
func SetToolHeader(name string) *HeaderOption {
	return &HeaderOption{Key: "x-gohome-mcp-tool", Value: name}
}

// SetResourceHeader returns a HeaderOption that adds the resource header.
func SetResourceHeader(uri string) *HeaderOption {
	return &HeaderOption{Key: "x-gohome-mcp-resource", Value: uri}
}

// Apply sets the header on req.
func (h *HeaderOption) Apply(req connect.AnyRequest) {
	if h != nil {
		req.Header().Set(h.Key, h.Value)
	}
}
