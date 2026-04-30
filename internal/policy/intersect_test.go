package policy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/gohome/internal/policy"
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
