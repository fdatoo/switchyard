package policy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/policy"
)

func TestTokenScope_PermitsAction_AllowList(t *testing.T) {
	s := policy.CompiledTokenScope{
		AllowedActions: map[policy.ActionKey]struct{}{
			"EntityService.Get":            {},
			"EntityService.List":           {},
			"EntityService.CallCapability": {},
		},
	}
	require.True(t, s.PermitsAction(policy.VerbRead, "EntityService.Get"))
	require.False(t, s.PermitsAction(policy.VerbAdmin, "ConfigService.Apply"))
}

func TestTokenScope_EmptyAllowedActions_PermitsAll(t *testing.T) {
	s := policy.CompiledTokenScope{}
	require.True(t, s.PermitsAction(policy.VerbRead, "EntityService.Get"))
	require.True(t, s.PermitsAction(policy.VerbAdmin, "AnythingService.Anything"))
}

func TestTokenScope_Allow_TrailingWildcardActions(t *testing.T) {
	s := policy.CompiledTokenScope{
		AllowedTools:    []string{"gohome__*"},
		AllowedServices: []string{"EntityService.*"},
	}
	require.True(t, s.Allow("gohome__turn_on", policy.Target{}))
	require.True(t, s.Allow("EntityService.Get", policy.Target{}))
	require.False(t, s.Allow("ConfigService.Apply", policy.Target{}))
}

func TestTokenScope_PermitsTarget_NarrowsByEntitySelector(t *testing.T) {
	s := policy.CompiledTokenScope{
		AllowedTargets: policy.CompiledSelector{
			ClassSet: map[string]struct{}{"Light": {}},
		},
	}
	require.True(t, s.PermitsTarget(policy.Target{Class: "Light"}))
	require.False(t, s.PermitsTarget(policy.Target{Class: "Lock"}))
}

func TestTokenScope_EmptyAllowedTargets_PermitsAll(t *testing.T) {
	s := policy.CompiledTokenScope{}
	require.True(t, s.PermitsTarget(policy.Target{Class: "Anything"}))
}

func TestTokenScope_Allow_NarrowsByArea(t *testing.T) {
	s := policy.CompiledTokenScope{
		AllowedServices: []string{"EntityService.*"},
		AllowedTargets: policy.CompiledSelector{
			AreaSet: map[policy.AreaSlug]struct{}{"kitchen": {}},
		},
	}
	require.True(t, s.Allow("EntityService.Get", policy.Target{Area: "kitchen"}))
	require.False(t, s.Allow("EntityService.Get", policy.Target{Area: "garage"}))
}

func TestTokenScope_Contains_RejectsBroaderChild(t *testing.T) {
	parent := policy.CompiledTokenScope{
		AllowedServices: []string{"EntityService.Get"},
		AllowedTargets: policy.CompiledSelector{
			AreaSet: map[policy.AreaSlug]struct{}{"kitchen": {}},
		},
	}
	require.True(t, parent.Contains(policy.CompiledTokenScope{
		AllowedServices: []string{"EntityService.Get"},
		AllowedTargets: policy.CompiledSelector{
			AreaSet: map[policy.AreaSlug]struct{}{"kitchen": {}},
		},
	}))
	require.False(t, parent.Contains(policy.CompiledTokenScope{
		AllowedServices: []string{"EntityService.*"},
		AllowedTargets: policy.CompiledSelector{
			AreaSet: map[policy.AreaSlug]struct{}{"kitchen": {}},
		},
	}))
	require.False(t, parent.Contains(policy.CompiledTokenScope{}))
}
