package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/cli"
)

func TestMCPToolsHuman(t *testing.T) {
	root := cli.NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"mcp", "tools"})
	err := root.Execute()
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "gohome__get_state")
	require.Contains(t, out, "gohome__write_config_file")
	require.Contains(t, out, "UNIMPLEMENTED")
}

func TestMCPToolsJSON(t *testing.T) {
	root := cli.NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"mcp", "tools", "--json"})
	err := root.Execute()
	require.NoError(t, err)
	var cat []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &cat))
	require.Len(t, cat, 12)
	// Check 2 admin tools
	admins := 0
	for _, td := range cat {
		if td["verb"] == "admin" {
			admins++
		}
	}
	require.Equal(t, 2, admins)
	// Check apply_scene is unimplemented
	for _, td := range cat {
		if td["name"] == "gohome__apply_scene" {
			require.Equal(t, "unimplemented", td["status"])
		}
	}
}
