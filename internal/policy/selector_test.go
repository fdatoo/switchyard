package policy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/policy"
)

func TestSelector_Matches_AreaSet(t *testing.T) {
	sel := policy.CompiledSelector{
		AreaSet: map[policy.AreaSlug]struct{}{"kitchen": {}, "living_room": {}},
	}
	require.True(t, policy.SelectorMatches(sel, policy.Target{Area: "kitchen"}))
	require.False(t, policy.SelectorMatches(sel, policy.Target{Area: "garage"}))
}

func TestSelector_Matches_ClassSet(t *testing.T) {
	sel := policy.CompiledSelector{
		ClassSet: map[string]struct{}{"Light": {}, "Switch": {}},
	}
	require.True(t, policy.SelectorMatches(sel, policy.Target{Class: "Light"}))
	require.False(t, policy.SelectorMatches(sel, policy.Target{Class: "Lock"}))
}

func TestSelector_Matches_EntitySet(t *testing.T) {
	sel := policy.CompiledSelector{
		EntitySet: map[string]struct{}{"lock.front_door": {}},
	}
	require.True(t, policy.SelectorMatches(sel, policy.Target{ID: "lock.front_door"}))
	require.False(t, policy.SelectorMatches(sel, policy.Target{ID: "lock.back_door"}))
}

func TestSelector_Matches_MatchAny(t *testing.T) {
	sel := policy.CompiledSelector{MatchAny: true}
	require.True(t, policy.SelectorMatches(sel, policy.Target{ID: "anything"}))
}

func TestSelector_Matches_EmptyMatchesNothing(t *testing.T) {
	sel := policy.CompiledSelector{}
	require.False(t, policy.SelectorMatches(sel, policy.Target{ID: "anything"}))
}

func TestExpandAreas_HierarchicalParentExpandsToDescendants(t *testing.T) {
	tree := policy.AreaTree{
		Children: map[policy.AreaSlug][]policy.AreaSlug{
			"main_floor":  {"kitchen", "living_room"},
			"kitchen":     {"pantry"},
			"living_room": nil,
			"pantry":      nil,
		},
	}
	set := policy.ExpandAreas(tree, []string{"main_floor"})
	require.Equal(t, map[policy.AreaSlug]struct{}{
		"main_floor": {}, "kitchen": {}, "living_room": {}, "pantry": {},
	}, set)
}

func TestExpandAreas_StarMatchesAll(t *testing.T) {
	tree := policy.AreaTree{
		Children: map[policy.AreaSlug][]policy.AreaSlug{"a": nil, "b": nil},
	}
	set := policy.ExpandAreas(tree, []string{"*"})
	require.Contains(t, set, policy.AreaSlug("a"))
	require.Contains(t, set, policy.AreaSlug("b"))
	require.Len(t, set, 2)
}

func TestExpandAreas_UnknownAreaIgnored(t *testing.T) {
	tree := policy.AreaTree{Children: map[policy.AreaSlug][]policy.AreaSlug{"a": nil}}
	set := policy.ExpandAreas(tree, []string{"ghost"})
	require.Empty(t, set)
}
