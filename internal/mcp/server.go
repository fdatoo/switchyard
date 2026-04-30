package mcp

import (
	"context"
	"fmt"
	"io"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run blocks running the stdio MCP server until ctx is canceled or stdin EOF.
func Run(ctx context.Context, deps Deps) error {
	opts := deps.ServerOpts
	if opts == nil {
		opts = &sdk.ServerOptions{}
	}
	server := sdk.NewServer(&sdk.Implementation{
		Name:    "gohome",
		Version: deps.Version,
	}, opts)

	// Register tools and resources via the injected callback (avoids import cycle).
	if deps.RegisterTools != nil {
		deps.RegisterTools(server)
	}

	var transport sdk.Transport
	if deps.Stdin != nil || deps.Stdout != nil {
		r := deps.Stdin
		if r == nil {
			r = io.NopCloser(nil)
		}
		w := deps.Stdout
		if w == nil {
			w = io.Discard
		}
		transport = &sdk.IOTransport{
			Reader: toReadCloser(r),
			Writer: toWriteCloser(w),
		}
	} else {
		transport = &sdk.StdioTransport{}
	}

	if err := server.Run(ctx, transport); err != nil {
		return fmt.Errorf("mcp serve: %w", err)
	}
	return nil
}

// toReadCloser wraps r in a nop closer if it doesn't already implement io.ReadCloser.
func toReadCloser(r io.Reader) io.ReadCloser {
	if rc, ok := r.(io.ReadCloser); ok {
		return rc
	}
	return io.NopCloser(r)
}

// toWriteCloser wraps w in a nop closer if it doesn't already implement io.WriteCloser.
func toWriteCloser(w io.Writer) io.WriteCloser {
	if wc, ok := w.(io.WriteCloser); ok {
		return wc
	}
	return nopWriteCloser{w}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
