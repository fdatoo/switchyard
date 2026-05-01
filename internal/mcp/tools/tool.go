// Package tools registers all gohome MCP tools.
package tools

import (
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/mcp/audit"
)

// Deps is the set of dependencies the tool registry needs.
type Deps struct {
	Server    *sdk.Server
	Client    *mcp.Client
	ConfigDir string
	MCPCaps   mcp.MCPCaps
	SessionID string
	Auth      auth.Authorizer
	Audit     *audit.Recorder
}

// ToolError is returned by handlers; wraps a reason code for tests.
type ToolError struct {
	Reason  string
	Message string
	Cause   error
}

func (e *ToolError) Error() string { return e.Reason + ": " + e.Message }
func (e *ToolError) Unwrap() error { return e.Cause }

// toToolError converts any error into a ToolError via the MCP envelope.
func toToolError(err error) error {
	env := mcp.ToMCPErrorEnvelope(err)
	return &ToolError{Reason: env.Reason, Message: env.Message, Cause: err}
}

// Register adds all tool handlers to the MCP server.
func Register(d Deps) {
	registerEntities(d)
	registerEvents(d)
	registerScenes(d)
	registerScripts(d)
	registerConfig(d)
	registerFiles(d)
}
