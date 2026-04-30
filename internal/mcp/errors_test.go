package mcp_test

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/mcp"
)

func TestToMCPError_PlainGoError(t *testing.T) {
	env := mcp.ToMCPErrorEnvelope(errors.New("oops"))
	require.Equal(t, "internal", env.Reason)
	require.Contains(t, env.Message, "oops")
}

func TestToMCPError_CodeWithoutDetail(t *testing.T) {
	cerr := connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))
	env := mcp.ToMCPErrorEnvelope(cerr)
	require.Equal(t, "unimplemented", env.Reason)
}

func TestToMCPError_NotFound(t *testing.T) {
	cerr := connect.NewError(connect.CodeNotFound, errors.New("missing"))
	env := mcp.ToMCPErrorEnvelope(cerr)
	require.Equal(t, "not_found", env.Reason)
}
