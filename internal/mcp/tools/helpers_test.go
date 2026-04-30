package tools_test

import (
	"context"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

var testImpl = &sdk.Implementation{Name: "test", Version: "0"}

// callTool connects an in-memory client/server pair and calls a tool.
func callTool(t *testing.T, s *sdk.Server, toolName string, args map[string]any) (*sdk.CallToolResult, error) {
	t.Helper()
	ct, st := sdk.NewInMemoryTransports()
	_, err := s.Connect(context.Background(), st, nil)
	require.NoError(t, err, "server Connect")

	client := sdk.NewClient(testImpl, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err, "client Connect")
	t.Cleanup(func() { cs.Close() })

	return cs.CallTool(context.Background(), &sdk.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
}

// textContent extracts the text from the first content element.
func textContent(t *testing.T, r *sdk.CallToolResult) string {
	t.Helper()
	require.NotNil(t, r)
	require.NotEmpty(t, r.Content, "expected at least one content element")
	tc, ok := r.Content[0].(*sdk.TextContent)
	require.True(t, ok, "expected TextContent, got %T", r.Content[0])
	return tc.Text
}
