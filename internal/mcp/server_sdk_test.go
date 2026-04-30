package mcp_test

import (
	"context"
	"io"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/mcp"
)

func TestRun_RegisterToolsCalled(t *testing.T) {
	pr, pw := io.Pipe()
	_ = pw.Close() // immediate EOF

	called := false
	done := make(chan error, 1)
	go func() {
		done <- mcp.Run(context.Background(), mcp.Deps{
			Stdin:   pr,
			Stdout:  io.Discard,
			Stderr:  io.Discard,
			Version: "test",
			RegisterTools: func(s *sdk.Server) {
				called = true
			},
		})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
	require.True(t, called)
}
