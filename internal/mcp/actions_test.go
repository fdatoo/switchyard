package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/mcp"
)

func TestToolActions_AllPresent(t *testing.T) {
	expected := []string{
		"gohome__get_state", "gohome__list_entities", "gohome__call_capability",
		"gohome__query_events", "gohome__tail_events", "gohome__apply_scene",
		"gohome__run_script", "gohome__validate_config", "gohome__apply_config",
		"gohome__eval_starlark", "gohome__read_config_file", "gohome__write_config_file",
	}
	for _, name := range expected {
		a, ok := mcp.ToolActions[name]
		require.True(t, ok, "missing: %s", name)
		require.NotEmpty(t, a.Verb)
	}
}

func TestToolActions_VerbCounts(t *testing.T) {
	counts := map[string]int{}
	for _, a := range mcp.ToolActions {
		counts[a.Verb]++
	}
	require.Equal(t, 6, counts["read"])
	require.Equal(t, 4, counts["call"])
	require.Equal(t, 2, counts["admin"])
}
