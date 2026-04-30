package policy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/policy"
)

func makeRoleGraph() policy.RoleGraph {
	return policy.RoleGraph{
		Roles: map[policy.RoleSlug]policy.RoleNode{
			"admin":  {Slug: "admin", Inherits: nil},
			"member": {Slug: "member", Inherits: nil},
			"guest":  {Slug: "guest", Inherits: nil},
			"kids":   {Slug: "kids", Inherits: []policy.RoleSlug{"guest"}},
		},
	}
}

func makeAreaTree() policy.AreaTree {
	return policy.AreaTree{
		Children: map[policy.AreaSlug][]policy.AreaSlug{
			"kids_floor":  {"nora_room", "milo_room"},
			"nora_room":   nil,
			"milo_room":   nil,
			"front_entry": nil,
		},
	}
}

func makeActionCatalog() policy.ActionCatalog {
	return policy.ActionCatalog{
		Actions: []policy.CatalogAction{
			{Service: "EntityService", Method: "List", Verb: policy.VerbRead, IsEntity: true},
			{Service: "EntityService", Method: "Get", Verb: policy.VerbRead, IsEntity: true},
			{Service: "EntityService", Method: "CallCapability", Verb: policy.VerbCall, IsEntity: true},
			{Service: "EntityService", Method: "Subscribe", Verb: policy.VerbRead, IsEntity: true},
			{Service: "ConfigService", Method: "Apply", Verb: policy.VerbAdmin, IsEntity: false},
		},
	}
}

func TestCompiler_HappyPath_ProducesAllowlistAndRules(t *testing.T) {
	rules := []policy.RawPolicy{
		{
			Name:     "admin_full",
			Subjects: []policy.RoleSlug{"admin"},
			Allow:    []policy.RawRule{{Targets: policy.RawSelector{MatchAny: true}}},
		},
		{
			Name:     "kids_bedrooms_only",
			Subjects: []policy.RoleSlug{"kids"},
			Allow: []policy.RawRule{{
				Verbs:   []policy.Verb{policy.VerbRead, policy.VerbCall},
				Targets: policy.RawSelector{Areas: []string{"kids_floor"}},
			}},
			Deny: []policy.RawRule{{
				Verbs:   []policy.Verb{policy.VerbCall},
				Targets: policy.RawSelector{Classes: []string{"Lock", "Alarm"}},
			}},
		},
	}
	c, err := policy.Compile(rules, makeRoleGraph(), makeAreaTree(), makeActionCatalog())
	require.NoError(t, err)
	require.True(t, c.PermitsAction("admin", policy.VerbRead, "EntityService.Get"))
	require.True(t, c.PermitsAction("admin", policy.VerbAdmin, "ConfigService.Apply"))
	require.True(t, c.PermitsAction("kids", policy.VerbRead, "EntityService.Get"))
	require.True(t, c.PermitsAction("kids", policy.VerbCall, "EntityService.CallCapability"))
	require.False(t, c.PermitsAction("kids", policy.VerbAdmin, "ConfigService.Apply"))
}

func TestCompiler_HierarchicalAreaExpansion(t *testing.T) {
	rules := []policy.RawPolicy{{
		Name:     "kids_bedrooms_only",
		Subjects: []policy.RoleSlug{"kids"},
		Allow:    []policy.RawRule{{Targets: policy.RawSelector{Areas: []string{"kids_floor"}}}},
	}}
	c, err := policy.Compile(rules, makeRoleGraph(), makeAreaTree(), makeActionCatalog())
	require.NoError(t, err)
	rule := c.AllowRules["kids"][0]
	require.Contains(t, rule.Targets.AreaSet, policy.AreaSlug("kids_floor"))
	require.Contains(t, rule.Targets.AreaSet, policy.AreaSlug("nora_room"))
	require.Contains(t, rule.Targets.AreaSet, policy.AreaSlug("milo_room"))
	require.NotContains(t, rule.Targets.AreaSet, policy.AreaSlug("front_entry"))
}

func TestCompiler_RejectsUnknownActionInService(t *testing.T) {
	rules := []policy.RawPolicy{{
		Name:     "bad",
		Subjects: []policy.RoleSlug{"admin"},
		Allow:    []policy.RawRule{{Services: []string{"EntityService"}}},
	}}
	rules2 := []policy.RawPolicy{{
		Name:     "bad",
		Subjects: []policy.RoleSlug{"admin"},
		Allow:    []policy.RawRule{{Services: []string{"GhostService"}}},
	}}
	_, err := policy.Compile(rules, makeRoleGraph(), makeAreaTree(), makeActionCatalog())
	require.NoError(t, err)
	_, err = policy.Compile(rules2, makeRoleGraph(), makeAreaTree(), makeActionCatalog())
	require.Error(t, err)
}

func TestCompiler_RejectsRoleInheritanceCycle(t *testing.T) {
	cyc := policy.RoleGraph{
		Roles: map[policy.RoleSlug]policy.RoleNode{
			"a": {Slug: "a", Inherits: []policy.RoleSlug{"b"}},
			"b": {Slug: "b", Inherits: []policy.RoleSlug{"a"}},
		},
	}
	_, err := policy.Compile(nil, cyc, policy.AreaTree{}, policy.ActionCatalog{})
	require.Error(t, err)
}

func TestCompiler_DenyOnNonEntityActionWithTargetsRejected(t *testing.T) {
	rules := []policy.RawPolicy{{
		Name:     "bad",
		Subjects: []policy.RoleSlug{"admin"},
		Deny: []policy.RawRule{{
			Services: []string{"ConfigService"},
			Targets:  policy.RawSelector{Classes: []string{"Light"}},
		}},
	}}
	_, err := policy.Compile(rules, makeRoleGraph(), makeAreaTree(), makeActionCatalog())
	require.Error(t, err)
}
