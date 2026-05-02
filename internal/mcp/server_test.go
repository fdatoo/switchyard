package mcp_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/mcp"
)

func TestRun_ExitsOnStdinEOF(t *testing.T) {
	pr, pw := io.Pipe()
	_ = pw.Close() // immediate EOF

	done := make(chan error, 1)
	go func() {
		done <- mcp.Run(context.Background(), mcp.Deps{
			Stdin:   pr,
			Stdout:  io.Discard,
			Stderr:  io.Discard,
			Version: "test",
		})
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit on stdin EOF")
	}
}
