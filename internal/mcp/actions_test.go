package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/mcp"
)

func TestToolActions_AllPresent(t *testing.T) {
	expected := []string{
		"switchyard__get_state", "switchyard__list_entities", "switchyard__call_capability",
		"switchyard__query_events", "switchyard__tail_events", "switchyard__apply_scene",
		"switchyard__run_script", "switchyard__validate_config", "switchyard__apply_config",
		"switchyard__eval_starlark", "switchyard__read_config_file", "switchyard__write_config_file",
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
