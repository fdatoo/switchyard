package mcp

import (
	"io"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fdatoo/switchyard/internal/observability"
)

// MCPCaps mirrors api.MCPConfig for use in the mcp package.
type MCPCaps struct {
	EvalResultMaxBytes       uint32
	ReadFileMaxBytes         uint32
	EntitySubscriptionBuffer uint32
	TraceSubscriptionBuffer  uint32
	TailDefaultWaitSeconds   uint32
	TailMaxWaitSeconds       uint32
}

// Deps is the closure of inputs the MCP server needs at construction time.
type Deps struct {
	Client    *Client
	ConfigDir string
	MCPCaps   MCPCaps
	SessionID string
	Version   string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer

	// Metrics for resource subscription telemetry.
	Metrics *observability.Metrics

	// ServerOpts, if non-nil, is merged with subscription handlers set up by
	// resources. Callers may set other fields (e.g. Logger, Instructions).
	ServerOpts *sdk.ServerOptions

	// RegisterTools, if set, is called during Run to register tool and resource
	// handlers onto the server before it starts accepting connections. This
	// avoids an import cycle between the mcp and mcp/tools packages.
	RegisterTools func(s *sdk.Server)
}
