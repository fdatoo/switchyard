package mcp

import (
	"log/slog"
	"net/http"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPConfig configures the MCP HTTP transport.
type HTTPConfig struct {
	SessionIdleTimeout time.Duration // 0 = no timeout
	Logger             *slog.Logger  // nil = no log
}

// NewHTTPHandler returns an http.Handler that serves the MCP Streamable HTTP
// transport. Mount it at /mcp in the listener.
func NewHTTPHandler(deps Deps, cfg HTTPConfig) http.Handler {
	opts := deps.ServerOpts
	if opts == nil {
		opts = &sdk.ServerOptions{}
	}
	server := sdk.NewServer(&sdk.Implementation{
		Name:    "switchyard",
		Version: deps.Version,
	}, opts)
	if deps.RegisterTools != nil {
		deps.RegisterTools(server)
	}
	return sdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdk.Server { return server },
		&sdk.StreamableHTTPOptions{
			Logger:         cfg.Logger,
			SessionTimeout: cfg.SessionIdleTimeout,
		},
	)
}
